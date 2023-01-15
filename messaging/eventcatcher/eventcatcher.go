package eventcatcher

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sasha-s/go-deadlock"
	"github.com/spf13/cast"
	"github.com/stackerstan/go-nostr"
	"mindmachine/auxiliarium/patches"
	"mindmachine/auxiliarium/samizdat"
	"mindmachine/consensus/conductor"
	"mindmachine/consensus/messagepack"
	"mindmachine/consensus/mindstate"
	"mindmachine/consensus/sequence"
	"mindmachine/consensus/shares"
	"mindmachine/database"
	"mindmachine/messaging/blocks"
	"mindmachine/messaging/nostrelay"
	"mindmachine/mindmachine"
	"mindmachine/scumclass/eventbucket"
)

func Start(terminate chan struct{}, wg *sync.WaitGroup) {
	mindmachine.LogCLI("Starting the Event catcher", 4)
	//find out what block we are starting with
	var block mindmachine.Event
	if b, ok := getLastSealedBlock(); ok {
		block = b
	} else {
		block = nostrelay.GetIgnitionBlock()
	}
	mindmachine.SetCurrentlyProcessing(block)
	// Start the messagepacker database first so that it's ready to handle any messages created as soon as they appear
	messagepack.StartBlock(block)
	conductor.Start(terminate, wg)
	//_, ok := nostrelay.FetchEventPack(test)
	//if ok {
	//	mindmachine.LogCLI("it worked", 3)
	//	mindmachine.Shutdown()
	//}
	mindmachine.PruneDeadOptionalRelays()
	if !samizdatStarted {
		subscribeToSamizdat()
	}
	go SubscribeToAllEvents(terminate, wg)
	fetchEventPackLooper()
	if blocksBehind() > 1 {
		if shares.VotePowerForAccount(mindmachine.MyWallet().Account) > 0 && mindmachine.MakeOrGetConfig().GetBool("forceBlocks") {
			catchUpOnBlocks()
		} else {
			mindmachine.LogCLI("looks like the network has stalled and will require someone with Votepower > 0 to recover it. Check the Samizdat tree, telegram group, or try running again later.", 1)
			mindmachine.Shutdown()
		}
	}
	go func() {
		catchUpOnBlocks()
		go startEventSubscription()
	}()
}

var fetching bool
var caughtUpWith []string

var mergedPatch bool

func blocksBehind() int64 {
	latest, ok := blocks.FetchLatestBlock()
	if ok {
		return latest.Height - mindmachine.CurrentState().Processing.Height
	}
	mindmachine.LogCLI("could not get current Bitcoin tip, check block server", 1)
	if !mindmachine.MakeOrGetConfig().GetBool("highly_reliable") {
		mindmachine.Shutdown()
		return 0
	}
	return 1
}

func catchUpOnBlocks() bool {
	latest, ok := blocks.FetchLatestBlock()
	if ok {
		if latest.Height-mindmachine.CurrentState().Processing.Height == 0 {
			return true
		}
		for latest.Height-mindmachine.CurrentState().Processing.Height > 0 {
			next, err := blocks.FetchBlock(mindmachine.CurrentState().Processing.Height + 1)
			if err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			tempBlock := nostr.Event{
				PubKey:    mindmachine.MyWallet().Account,
				CreatedAt: time.Now(),
				Kind:      125,
				Tags:      nostr.Tags{[]string{"block", fmt.Sprintf("%d", next.Height), next.Hash, fmt.Sprintf("%d", next.Time)}},
			}
			err = tempBlock.Sign(mindmachine.MyWallet().PrivateKey)
			if err != nil {
				mindmachine.LogCLI(err.Error(), 0)
			}
			if _, ok := handleBlockHeader(mindmachine.ConvertToInternalEvent(&tempBlock), true); ok {
				//success
			}
			time.Sleep(time.Second * 2)
		}
		return catchUpOnBlocks()
	}
	return false
}

var attempts = 0

func fetchEventPackLooper() {
	startHeight := mindmachine.CurrentState().Processing.Height
	fetchLatest640001()
	if startHeight == mindmachine.CurrentState().Processing.Height {
		if mindmachine.CurrentState().Processing.Height == 761151 {
			fetchLatest640001()
		}
		if blocksBehind() > 1 && attempts < 4 {
			time.Sleep(time.Second * 2)
			attempts++
			fetchEventPackLooper()
		}
		if blocksBehind() > 1 && attempts == 4 {
			mindmachine.LogCLI("we might be having problems fetching events from relays", 1)
		}
		return
	}
	fetchEventPackLooper()
}

//todo return the height of the resulting state, and if it isn't the current bitcoin tip try again. If we have votepower and we can't get any higher, start populating blocks.
//whenever we fall behind the current tip, try to fetch event pack again
func fetchLatest640001() {
	//todo: also search relaysOptional if relaysMust fails
	//return
	mindmachine.LogCLI("Looking for a kind 640001 Event to catch up to our peers", 4)
	if !fetching {
		var states []mindmachine.HashSeq
		fetching = true
		defer func() { fetching = false }()
		liveEventsMutex.Lock()
		defer liveEventsMutex.Unlock()
		//pool := nostrelay.NewRelayPool()
		pool := nostr.NewRelayPool()
		mindmachine.LogCLI("Connecting to relay pool", 3)
		var relays []string
		relays = append(relays, mindmachine.MakeOrGetConfig().GetStringSlice("relaysMust")...)
		//relays = append(relays, mindmachine.MakeOrGetConfig().GetStringSlice("relaysOptional")...)
		for _, s := range relays {
			errchan := pool.Add(s, nostr.SimplePolicy{Read: true, Write: true})
			go func(s string) {
				for err := range errchan {
					e := fmt.Sprintf("rvyg65: %s", err.Error())
					mindmachine.LogCLI(fmt.Sprintf("%s %s", e, s), 2)
				}
			}(s)
		}
		//defer func() {
		//	for _, s := range mindmachine.MakeOrGetConfig().GetStringSlice("relaysMust") {
		//		pool.Remove(s)
		//	}
		//}()

		accounts := shares.AccountsWithVotepower()
		var accs []mindmachine.Account
		for account, _ := range accounts {
			accs = append(accs, account)
		}
		t := time.Now()
		t = t.Add(-4 * time.Hour)
		if shares.VotePowerForAccount(mindmachine.MyWallet().Account) > 0 {
			//t = t.Add(-168 * time.Hour)
		}
		_, evnts, unsub := pool.Sub(nostr.Filters{nostr.Filter{
			Kinds:   []int{640001},
			Authors: accs,
			Since:   &t,
		}})
		var tries int64 = 0
		//behind := blocksBehind()
		gotResult := false
		var events = make(map[string]nostr.Event)
		//for s, _ := range pool.Relays {
		//	mindmachine.LogCLI(fmt.Sprintf("Connected to %s", s), 4)
		//} fiatjaf broke userspace! First he gives us branle, and now this?
	L:
		for tries < 10 {
			select {
			case e := <-nostr.Unique(evnts):
				//fmt.Println(e)
				events[e.ID] = e
				gotResult = true
			case <-time.After(time.Second * 2):
				if gotResult {
					break L
				}
				tries++
			}
		}
		unsub()
		// sub.Unsub() this makes us crash:
		//github.com/fiatjaf/go-nostr.Subscription.startHandlingUnique({{0xc000916430, 0xe}, 0xc000926690, {0xc0002740e0, 0x1, 0x1}, 0xc000322ae0, 0x0, 0xc000322b40, 0x0})
		//	/Users/x/go/pkg/mod/github.com/fiatjaf/go-nostr@v0.7.4/subscription.go:66 +0x198
		//created by github.com/fiatjaf/go-nostr.(*Subscription).Sub
		//	/Users/x/go/pkg/mod/github.com/fiatjaf/go-nostr@v0.7.4/subscription.go:54 +0x225
		//exit status 2
		//make: *** [run] Error 1
		var unmarshalled = make(map[string]mindstate.Kind640001)
		for _, event := range events {
			u := mindstate.Kind640001{}
			err := json.Unmarshal([]byte(event.Content), &u)
			if err != nil {
				fmt.Println(event.Content)
				mindmachine.LogCLI(err.Error(), 1)
			} else {
				unmarshalled[event.ID] = u
			}
		}
		var largestSeq int64
		var highestBlock int64
		var idHighestSequence string
		var idHighestBlock string
		for p, kind640001 := range unmarshalled {
			var seq int64
			for _, state := range kind640001.LatestState {
				seq += state.Sequence
			}
			if kind640001.Height >= highestBlock {
				highestBlock = kind640001.Height
				idHighestBlock = p
				if seq >= largestSeq {
					largestSeq = seq
					idHighestSequence = p
				}
			}
		}
		if idHighestBlock != idHighestSequence {
			fmt.Printf("\nHighest Sequence:\n%#v\n\nHighest Block: \n%#v\n", unmarshalled[idHighestSequence], unmarshalled[idHighestBlock])
		}
		//idHighestSequence = "74e66d1a913f968c84a8afe176327ed91ac5bac92ef3c357c328ed04a1421cf7" //"edafbe657131fabc758f77a1fc7933a671b6fc542b97821235b98f7b8dd11f8e" //debug
		highestSequenceEventPack := unmarshalled[idHighestSequence]
		var alreadyCaughtUp bool
		if mindmachine.Contains(caughtUpWith, highestSequenceEventPack.OpReturn) || mindmachine.Contains(caughtUpWith, idHighestSequence) {
			alreadyCaughtUp = true
		}
		if highestSequenceEventPack.Height > mindmachine.CurrentState().Processing.Height && !alreadyCaughtUp {
			mindmachine.LogCLI(fmt.Sprintf("Attempting to rebuild state from Event: %s at height %d", idHighestSequence, unmarshalled[idHighestSequence].Height), 4)
			if orderedEventsToReplay, ok := nostrelay.FetchEventPack(highestSequenceEventPack.EventIDs); ok {
				var failed bool
				//todo add in any missing blocks between two block heights
				//for creating: intercept current eventpack and remove all blocks between one block buffer between events which have blocks on one boundary

				hashseqs := handleEventPack(orderedEventsToReplay)
				for s, state := range highestSequenceEventPack.LatestState {
					if mindstate.GetLatestStates()[s].State != state.State {
						fmt.Printf("\nours\n%#v\n\ntheirs\n%#v\n\n", mindstate.GetLatestStates()[s].State, state.State)
						fmt.Printf("\nours\n%#v\n\ntheirs\n%#v\n\n", mindstate.GetLatestStates(), highestSequenceEventPack.LatestState)
						mindmachine.LogCLI("failed to get to the same state as the eventpack creator, try running make reset and see if it works next time, alternatively there are a few things we could try if this happens, go look at the eventcatcher code", 2)
						failed = true
					}
				}
				opr := mindstate.OpReturn().OpReturn
				if opr != highestSequenceEventPack.OpReturn {
					mindmachine.LogCLI("We reached a different OP_RETURN to the one provided. We have: "+opr+" eventpack has: "+highestSequenceEventPack.OpReturn, 1)
					failed = true
				}
				if !failed {
					mindmachine.LogCLI(fmt.Sprintf("Successfully reproduce state: %s", idHighestSequence), 3)
					mindmachine.LogCLI("Current OP_RETURN: "+mindstate.OpReturn().OpReturn, 4)
					caughtUpWith = append(caughtUpWith, idHighestSequence, opr)
					states = append(states, hashseqs...)
				}
			} else {
				//mindmachine.LogCLI("failed to get event pack", 0)
			}
		}
		if shares.VotePowerForAccount(mindmachine.MyWallet().Account) > 0 {
			for _, state := range states {
				seq := sequence.GetSequence(mindmachine.MyWallet().Account)
				if vpssEvent, ok := hashSeqToSignedVPSS(state, seq+1); ok {
					localVpssEvent := mindmachine.ConvertToInternalEvent(&vpssEvent)
					if _, ok := conductor.HandleMessage(localVpssEvent); ok {
						nostrelay.PublishEvent(localVpssEvent.Nostr())
						messagepack.PackMessage(localVpssEvent)
					}
				}
			}
		}
		if mergedPatch {
			for s, repository := range patches.AllRepositories() {
				fmt.Println(s)
				m := repository.GetMapOfPatches()
				fmt.Printf("%#v\n", m)
				if repository.Name == "mindmachine" {
					fmt.Println(repository.BuildTip())
				}
			}
			mindmachine.LogCLI("A Patch has been Merged, please rebuild and restart", 2)
			mindmachine.Shutdown()
		}
	}
}

func Replay() {
	source := ""
	all := strings.Split(source, " ")
	if orderedEventsToReplay, ok := nostrelay.FetchEventPack(all); ok {
		handleEventPack(orderedEventsToReplay)
	}
}

var liveEventsMutex = &sync.Mutex{}
var started = false
var samizdatStarted = false

func subscribeToSamizdat() {
	if !samizdatStarted {
		samizdatStarted = true
		s, ok := nostrelay.FetchEventPack([]string{"9e333343184fe3e98b028782f7098cf596f1f46adf546541e7317d9a5f1d5d57"})
		if !ok {
			mindmachine.LogCLI("can't get first samizdat", 0)
			return
		}
		samizdat.HandleEvent(s[0])
		//pool := nostrelay.NewRelayPool()
		pool := nostr.NewRelayPool()
		mindmachine.LogCLI("Connecting to relay pool", 3)
		for _, s := range mindmachine.MakeOrGetConfig().GetStringSlice("relaysMust") {
			errchan := pool.Add(s, nostr.SimplePolicy{Read: true, Write: true})
			go func() {
				for err := range errchan {
					e := fmt.Sprintf("2f3kut9: %s", err.Error())
					mindmachine.LogCLI(e, 2)
				}
			}()
		}
		defer func() {
			for _, s := range mindmachine.MakeOrGetConfig().GetStringSlice("relaysMust") {
				pool.Remove(s)
			}
		}()

		filters := nostr.Filters{}
		tags := make(map[string][]string)
		tags["e"] = []string{"9e333343184fe3e98b028782f7098cf596f1f46adf546541e7317d9a5f1d5d57"}
		filters = append(filters, nostr.Filter{
			Kinds: []int{1},
			Tags:  tags,
		})
		_, evnts, unsub := pool.Sub(filters)
		go func() {
			for {
				select {
				case e := <-nostr.Unique(evnts):
					if ok, _ := e.CheckSignature(); ok {
						samizdat.HandleEvent(mindmachine.ConvertToInternalEvent(&e))
					}
				case <-time.After(time.Second * 5):
					unsub()
					break
				}
			}
		}()
	}
}

func SubscribeToAllEvents(terminate chan struct{}, wg *sync.WaitGroup) {
	eventbucket.StartDb(terminate, wg)
	if len(mindmachine.MakeOrGetConfig().GetStringSlice("relaysOptional")) < 1 {
		mindmachine.LogCLI("we do not have any optional relays, possible network problem", 2)
		mindmachine.Shutdown()
	}
	pool := nostr.NewRelayPool()
	mindmachine.LogCLI("Subscribing to Kind 1 Events", 3)
	relays := mindmachine.MakeOrGetConfig().GetStringSlice("relaysOptional")
	relays = append(relays, mindmachine.MakeOrGetConfig().GetStringSlice("relaysMust")...)
	for _, s := range relays {
		errchan := pool.Add(s, nostr.SimplePolicy{Read: true, Write: true})
		go func(s string) {
			for err := range errchan {
				e := fmt.Sprintf("q9u8mtx: %s", err.Error())
				mindmachine.LogCLI(fmt.Sprintf("%s %s", e, s), 2)
			}
		}(s)
	}

	filters := nostr.Filters{}
	filters = append(filters, nostr.Filter{
		//Kinds: []int{640001},
	})
	_, evnts, unsub := pool.Sub(filters)
	go func() {
	L:
		for {
			select {
			case e := <-nostr.Unique(evnts):
				if ok, _ := e.CheckSignature(); ok {
					eventbucket.HandleEvent(mindmachine.ConvertToInternalEvent(&e))
				}
			case <-terminate:
				unsub()
				break L
			case <-time.After(time.Second * 60):
				unsub()
				mindmachine.PruneDeadOptionalRelays()
				go SubscribeToAllEvents(terminate, wg)
				break L
			}
		}
	}()
}

func startEventSubscription() {
	if !started {
		started = true
		//pool := nostrelay.NewRelayPool()
		pool := nostr.NewRelayPool()
		mindmachine.LogCLI("Connecting to relay pool", 3)
		for _, s := range mindmachine.MakeOrGetConfig().GetStringSlice("relaysMust") {
			errchan := pool.Add(s, nostr.SimplePolicy{Read: true, Write: true})
			go func() {
				for err := range errchan {
					e := fmt.Sprintf("9u8034: %s", err.Error())
					mindmachine.LogCLI(e, 2)
				}
			}()
		}
		defer func() {
			for _, s := range mindmachine.MakeOrGetConfig().GetStringSlice("relaysMust") {
				pool.Remove(s)
			}
		}()

		latest := mindstate.GetLatestStates()
		block, err := blocks.FetchBlock(latest["shares"].Height)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 0)
		}
		//we want all events produced after the latest >500 permille block height,
		//but block timestamps can be up to 2 hours off
		t := time.Unix(block.Time-7200, 0)
		kinds := []int{}
		for i, _ := range mindmachine.GetAllKinds() {
			if i >= 640000 && i <= 649999 {
				kinds = append(kinds, int(i))
			}
		}
		filters := nostr.Filters{}
		filters = append(filters, nostr.Filter{
			Kinds: kinds,
			Since: &t,
		})
		_, evts, unsub := pool.Sub(filters)
		defer unsub()
		eventQueue := make(chan nostr.Event)
		go func() {
			for {
				select {
				case e := <-nostr.Unique(evts):
					//fmt.Printf("\n138\n%#v\n", e)
					go func() {
						eventQueue <- e
					}()
				}
			}
		}()
		blockChan := blocks.SubscribeToBlocks()
		frontendChan := nostrelay.SubscribeToMessages()
		frontEndBloom := mindmachine.MakeNewInverseBloomFilter(1000)
		//todo cache the events for below
		for {
			select {
			case <-time.After(time.Second * 60):
				if blocksBehind() > 0 {
					fetchEventPackLooper()
					if blocksBehind() > 1 {
						if shares.VotePowerForAccount(mindmachine.MyWallet().Account) > 0 {
							catchUpOnBlocks()
						} else {
							mindmachine.LogCLI("looks like the network has stalled and will require someone with Votepower > 0 to recover it", 1)
							mindmachine.Shutdown()
						}
					}
					catchUpOnBlocks()
				}
			case e := <-frontendChan:
				if !pushVpss(e) {
					frontEndBloom(e.ID)
					if e.Kind == 1 {
						samizdat.HandleEvent(e)
						nostrelay.PublishEvent(e.Nostr())
						continue
					}
					liveEventsMutex.Lock()
					mindmachine.LogCLI("Handling event from frontend: "+fmt.Sprint(e.ID), 4)
					if handleEventInLiveMode(e) {
						nostrelay.PublishEvent(e.Nostr())
					} else {
						fmt.Printf("\n%#v\n", e)
						mindmachine.LogCLI("An event from the frontend failed!", 2)
					}
					liveEventsMutex.Unlock()
					continue

				}
			case e := <-eventQueue:
				pushVpss(mindmachine.ConvertToInternalEvent(&e))
				if frontEndBloom(e.ID) { //!pushVpss(mindmachine.ConvertToInternalEvent(&e)) &&
					liveEventsMutex.Lock()
					if e.Kind == 1 {
						samizdat.HandleEvent((mindmachine.ConvertToInternalEvent(&e)))
					} else if handleEventInLiveMode(mindmachine.ConvertToInternalEvent(&e)) {
						mindmachine.LogCLI("Handled event from relay: "+fmt.Sprint(e.ID), 4)
					}
					liveEventsMutex.Unlock()
					continue
				}
			case bh := <-blockChan:
				liveEventsMutex.Lock()
				mindmachine.LogCLI("Got a block: "+fmt.Sprint(bh.Height), 4)
				tempBlock := nostr.Event{
					PubKey:    mindmachine.MyWallet().Account,
					CreatedAt: time.Now(),
					Kind:      125,
					Tags:      nostr.Tags{[]string{"block", fmt.Sprintf("%d", bh.Height), bh.Hash, fmt.Sprintf("%d", bh.Time)}},
				}
				err := tempBlock.Sign(mindmachine.MyWallet().PrivateKey)
				if err != nil {
					mindmachine.LogCLI(err.Error(), 0)
					return
				}
				if height, ok := handleBlockHeader(mindmachine.ConvertToInternalEvent(&tempBlock), true); ok {
					//opReturn := fmt.Sprintf("OP_RETURN: %s", mindstate.OpReturn().OpReturn)
					//mindmachine.LogCLI(opReturn, 4)
					//if shares.VotePowerForAccount(mindmachine.MyWallet().Account) > 0 {
					//	publishEventPack(opReturn)
					//}
				} else {
					if height > mindmachine.CurrentState().Processing.Height+1 {
						//mindmachine.PruneDeadOptionalRelays()
						go fetchLatest640001()
					}
				}
				liveEventsMutex.Unlock()
			}
		}
	}
}

var vpssBuffer []mindmachine.Event
var vpssBufferMutex = &sync.Mutex{}

func pushVpss(e mindmachine.Event) bool {
	if e.Kind == 640000 {
		vpssBufferMutex.Lock()
		defer vpssBufferMutex.Unlock()
		vpssBuffer = append(vpssBuffer, e)
		return true
	}
	return false
}

func popVpss() []mindmachine.Event {
	vpssBufferMutex.Lock()
	defer vpssBufferMutex.Unlock()
	cpy := []mindmachine.Event{}
	for _, event := range vpssBuffer {
		cpy = append(cpy, event)
	}
	vpssBuffer = []mindmachine.Event{}
	return cpy
}

var mm = &deadlock.Mutex{}

func handleEventInLiveMode(e mindmachine.Event) bool {
	mm.Lock()
	defer mm.Unlock()
	if h, ok := e.Height(); ok {
		if h == mindmachine.CurrentState().Processing.Height || h+1 == mindmachine.CurrentState().Processing.Height {
			if hs, ok := conductor.HandleMessage(e); ok {
				messagepack.PackMessage(e)
				//todo if we are missing events when we go to fetch eventpacks, and the missing event is a VPSS, it's probably because the Mind reported that an event was successful even though the Mind-state did not get updated (i.e. the event triggered an update that has already happened in the past and the Mind didn't reject it). Possible solution: every Mind MUST report a failure unless the hashseq is different to the current state. Or maybe use a persistent bloom filter here and validate that the new hashseq is actually new.

				if mind, ok := mindmachine.WhichMindForKind(e.Kind); ok && mind != "vpss" {
					if hs.Mind != "shares" {
						hs.NailedTo = shares.HashOfCurrentState() //or should we nail this to the latest >500 permille instead of just the latest?
					}
					sequence := sequence.GetSequence(mindmachine.MyWallet().Account)
					hs.EventID = e.ID
					if vpssEvent, vOk := hashSeqToSignedVPSS(hs, sequence+1); vOk {
						localVpssEvent := mindmachine.ConvertToInternalEvent(&vpssEvent)
						if !mindstate.RegisterState(localVpssEvent) {
							mindmachine.LogCLI("this should not happen", 0)
						}
						if shares.VotePowerForAccount(mindmachine.MyWallet().Account) > 0 {
							if _, lOk := conductor.HandleMessage(localVpssEvent); lOk {
								nostrelay.PublishEvent(vpssEvent)
								messagepack.PackMessage(localVpssEvent)
							} else {
								//todo move this error to vpss Mind so that we don't report failed vpss if it failed cause we've already signed it
								fmt.Printf("\nOUR NEWLY CREATED VPSS FAILED\n%#v\n", vpssEvent)
								//mindmachine.LogCLI("this should not happen: our VPSS failed locally", 0)
							}
						}
					}
				}
				return true
			}
		}
	}
	//todo handle messages for Scum Class Minds (does not have to be the right height or order)
	//mindmachine.LogCLI(fmt.Sprintf("Failed %s of kind %d", e.ID, e.Kind), 3)
	return false
}

func handleBlockHeader(e mindmachine.Event, livemode bool) (int64, bool) {
	if e.Kind == 125 {
		block, ok := e.GetTags("block")
		if ok {
			height, err := strconv.ParseInt(block[0], 10, 64)
			if err == nil {
				if height == mindmachine.CurrentState().Processing.Height+1 {
					for _, event := range popVpss() {
						if !livemode {
							mindmachine.LogCLI("we should have processed all eventpack VPSS in-line with the eventpack, this should not happen", 1)
						}
						//if event.PubKey != mindmachine.MyWallet().Account {
						//fmt.Println(309)
						handleEventInLiveMode(event)
						//}
					}
					messagepack.SealBlock(height - 1)
					opr := mindstate.OpReturn().OpReturn
					mindmachine.LogCLI(fmt.Sprintf("OP_RETURN: %s", opr), 4)
					if shares.VotePowerForAccount(mindmachine.MyWallet().Account) > 0 && livemode {
						publishEventPack(opr, false)
					}
					mindmachine.SetCurrentlyProcessing(e)
					messagepack.StartBlock(e)
					storeLastStartedBlock(e)
					return height, true
				} else {
					return height, false
				}
				//if height > mindmachine.CurrentState().Processing.Height+1 {
				//	//todo we are missing blocks
				//	//this should be checked upstream because if this was from a messagepack we need to error out, but if this was live, we need to look for messagepacks.
				//}
			} else {
				mindmachine.LogCLI(err.Error(), 1)
			}
		}
	}
	return 0, false
}

func publishEventPack(opreturn string, force bool) {
	mp := messagepack.GetMessagePacks(mindmachine.MakeOrGetConfig().GetInt64("ignitionHeight"))
	var newMp []string
	var lastBlock = mindmachine.MakeOrGetConfig().GetInt64("ignitionHeight") - 1
	var lastMessageTypeIsBlock = false
	var lastInsertedBlock int64
	for _, s := range mp {
		if len(s) == 64 {
			if lastMessageTypeIsBlock && lastInsertedBlock != lastBlock {
				newMp = append(newMp, fmt.Sprintf("%d", lastBlock))
				lastInsertedBlock = lastBlock
			}
			newMp = append(newMp, s)
			lastMessageTypeIsBlock = false
		}
		if len(s) < 7 {
			if height, err := cast.ToInt64E(s); err != nil {
				mindmachine.LogCLI(err, 1)
			} else {
				if height == lastBlock+1 {
					lastBlock = height
					if !lastMessageTypeIsBlock {
						newMp = append(newMp, fmt.Sprintf("%d", lastBlock))
						lastInsertedBlock = lastBlock
						lastMessageTypeIsBlock = true
					}
				}
			}
		}
	}
	if force {
		latestFromNetwork, ok := blocks.FetchLatestBlock()
		if ok {
			if latestFromNetwork.Height > lastBlock {
				newMp = append(newMp, fmt.Sprintf("%d", latestFromNetwork.Height))
				lastBlock = latestFromNetwork.Height
				lastInsertedBlock = lastBlock
			}

		}
	}
	if lastBlock != lastInsertedBlock {
		newMp = append(newMp, fmt.Sprintf("%d", lastBlock))
	}
	latest := mindstate.GetLatestStates()
	content := mindstate.Kind640001{
		Height:      mindmachine.CurrentState().Processing.Height,
		EventIDs:    newMp,
		LatestState: latest,
		OpReturn:    opreturn,
	}
	j, err := json.Marshal(content)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	n := nostr.Event{
		PubKey:    mindmachine.MyWallet().Account,
		CreatedAt: time.Now(),
		Kind:      640001,
		Tags:      nil,
		Content:   fmt.Sprintf("%s", j),
	}
	n.Sign(mindmachine.MyWallet().PrivateKey)
	nostrelay.PublishEvent(n)
	mindmachine.LogCLI("Published our current state", 4)
}

func handleEventPack(events []mindmachine.Event) (states []mindmachine.HashSeq) {
	//todo: problem: when shutting down we lose data because we keep processing events after databases have been closed
	//solution: hook into the terminate channel and stop processing events if terminate is called
	var skip bool
	for _, event := range events {
		if event.Kind == 125 {
			//even if it's not ok, we need the result so we know to skip this block or not
			height, _ := handleBlockHeader(event, false)
			if mindmachine.CurrentState().Processing.Height < height-1 {
				fmt.Printf("\n2345kt34\ncurrent state%d\nevent height: %d\n", mindmachine.CurrentState().Processing.Height, height)
				mindmachine.LogCLI("kewo84otn invalid block in messagepack", 0)
			}
			if mindmachine.CurrentState().Processing.Height > height {
				skip = true
				continue
			}
			if height == mindmachine.CurrentState().Processing.Height+1 {
				skip = false
			}
			continue
		}
		if !skip {
			if mind, ok := mindmachine.WhichMindForKind(event.Kind); ok {
				if hs, ok := conductor.HandleMessage(event); ok {
					messagepack.PackMessage(event)
					if mind != "vpss" {
						if hs.Mind != "shares" {
							hs.NailedTo = shares.HashOfCurrentState() //or should we nail this to the latest >500 permille instead of just the latest?
						}
						hs.EventID = event.ID
						seq := sequence.GetSequence(mindmachine.MyWallet().Account)
						if vpssEvent, ok := hashSeqToSignedVPSS(hs, seq+1); ok {
							localVpssEvent := mindmachine.ConvertToInternalEvent(&vpssEvent)
							if !mindstate.RegisterState(localVpssEvent) {
								mindmachine.LogCLI("this should not happen", 0)
							}
							states = append(states, hs)
							if event.Kind == 641004 {
								return
							}
						}
					}
				} else {
					fmt.Printf("\n%#v\n", event)
					mindmachine.LogCLI("event in messagepack failed, keep running \"make reset\" until it works (buggy)", 2)
					mindmachine.Shutdown()
				}
			}
		}
	}
	return
}

func storeLastStartedBlock(e mindmachine.Event) {
	b, err := json.MarshalIndent(e, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	database.Write("eventconductor", "latestblock", b)
}

func getLastSealedBlock() (mindmachine.Event, bool) {
	var block mindmachine.Event
	h, ok := database.Open("eventconductor", "latestblock")
	if ok {
		err := json.NewDecoder(h).Decode(&block)
		if err != nil {
			if err.Error() != "EOF" {
				mindmachine.LogCLI(err.Error(), 0)
			}
		}
		h.Close()
		return block, true
	}
	return mindmachine.Event{}, false
}

func hashSeqToSignedVPSS(hs mindmachine.HashSeq, sequence int64) (n nostr.Event, o bool) {
	if len(hs.Hash) == 64 {
		content := mindmachine.Kind640000{
			Mind:      hs.Mind,
			Hash:      hs.Hash,
			Sequence:  hs.Sequence,
			ShareHash: hs.NailedTo,
			Height:    hs.CreatedAt,
			EventID:   hs.EventID,
		}
		j, err := json.Marshal(content)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 0)
		}
		n.CreatedAt = time.Now()
		n.Kind = 640000
		n.PubKey = mindmachine.MyWallet().Account
		n.Tags = nostr.Tags{[]string{"height", fmt.Sprintf("%d", mindmachine.CurrentState().Processing.Height)}, []string{"sequence", fmt.Sprintf("%d", sequence)}}
		n.Content = fmt.Sprintf("%s", j)
		n.ID = n.GetID()
		err = n.Sign(mindmachine.MyWallet().PrivateKey)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 0)
		}
		o = true
	}
	return
}
