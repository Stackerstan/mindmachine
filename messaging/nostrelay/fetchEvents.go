package nostrelay

import (
	"fmt"
	"time"

	"github.com/sasha-s/go-deadlock"
	"github.com/spf13/cast"
	"github.com/stackerstan/go-nostr"
	"mindmachine/consensus/messagepack"
	"mindmachine/messaging/blocks"
	"mindmachine/mindmachine"
	"mindmachine/scumclass/eventbucket"
)

func FetchLocalCachedEvent(event string) (nostr.Event, bool) {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	if localEvent, ok := currentState.data[event]; ok {
		return localEvent, true
	}
	if localEvent, ok := eventbucket.Fetch(event); ok {
		return localEvent, true
	}
	return nostr.Event{}, false
}

func filterOutAnythingNotRequiredToFetchFromRelays(input []string) (output []string) {
	for _, s := range input {
		if len(s) == 64 {
			currentState.mutex.Lock()
			if _, ok := currentState.data[s]; !ok {
				output = append(output, s)
			}
			currentState.mutex.Unlock()
		}
	}
	return
}

func fetchEventsFromRelays(inputEvents []string, relayList []string) (events map[string]nostr.Event, missing []string, ok bool) {
	if len(relayList) < 1 || len(inputEvents) < 1 {
		fmt.Printf("\nYou requested %d events from %d relays\n", len(inputEvents), len(relayList))
		mindmachine.LogCLI("relayList and inputEvents must be greater than 0", 1)
		return
	}
	events = make(map[string]nostr.Event)
	if len(inputEvents) > 200 {
		//fmt.Printf("\nevent pack length: %d is too long\n", len(inputEvents))
		e, m, ok := fetchEventsFromRelays(inputEvents[:200], relayList)
		if !ok {
			return e, m, false
		}
		for _, event := range e {
			events[event.ID] = event
		}
		e, m, ok = fetchEventsFromRelays(inputEvents[200:], relayList)
		if !ok {
			return e, m, false
		}
		for _, event := range e {
			events[event.ID] = event
		}
		return events, missing, true
	}

	var temp = make(map[string]nostr.Event)
	var filters nostr.Filters
	filters = append(filters, nostr.Filter{IDs: inputEvents})
	pool := nostr.NewRelayPool()
	var relays []string
	relays = append(relays, relayList...)
	for _, s := range relays {
		mindmachine.LogCLI(fmt.Sprintf("Connecting to relay %s to fetch %d events", s, len(inputEvents)), 3)
		//fmt.Printf("\n\n%#v\n", inputEvents)
		errchan := pool.Add(s, nostr.SimplePolicy{Read: true, Write: true})
		go func(relay string) {
			for err := range errchan {
				e := fmt.Sprintf("j8453: %s", err.Error())
				mindmachine.LogCLI(e, 2)
				if mindmachine.Contains(mindmachine.MakeOrGetConfig().GetStringSlice("relaysMust"), relay) {
					report := fmt.Sprintf("We cannot reach %s try restarting with make reset and log an issue if this continues", relay)
					mindmachine.LogCLI(report, 2)
					mindmachine.Shutdown()
				}
			}
		}(s)
	}
	go func() {
		for n := range pool.Notices {
			e := fmt.Sprintf("f345: relay: %s notice: %s", n.Relay, n.Message)
			mindmachine.LogCLI(e, 4)
		}
	}()
	_, evts, unsub := pool.Sub(filters)
	//attempts := 0
E:
	for {
		select {
		case event := <-nostr.Unique(evts):
			if mindmachine.Contains(inputEvents, event.ID) {
				temp[event.ID] = event
				if len(inputEvents) == len(temp) {
					break E
				}
			}
			continue
		case <-time.After(time.Second * 10):
			if len(inputEvents) == len(temp) {
				break E
			}
			fmt.Println("y078345")
			fmt.Println(len(inputEvents))
			fmt.Println(len(temp))
			break E
			//if len(inputEvents)+1 < attempts {
			//	break E
			//}
			//attempts++
			//continue
		}
	}
	unsub()

	if len(inputEvents) != len(temp) {
		var failed []string
		for _, s := range inputEvents {
			if _, ok := temp[s]; !ok {
				failed = append(failed, s)
			}
		}
		time.Sleep(time.Second * 5) //give time for transient network problems to recover
		e, m, ok := fetchEventsFromRelays(failed, relayList)
		if ok {
			for _, event := range e {
				temp[event.ID] = event
			}
		} else {
			return events, m, false
		}
	}
	for _, id := range inputEvents {
		n, ok := temp[id]
		if ok {
			events[n.ID] = n
		}
	}
	return events, []string{}, true
}

func FetchEventPack(eventPack []string) (events []mindmachine.Event, k bool) {
	relays := mindmachine.MakeOrGetConfig().GetStringSlice("relaysMust")
	fetch := filterOutAnythingNotRequiredToFetchFromRelays(eventPack)
	if len(fetch) > 0 {
		e, missing, _ := fetchEventsFromRelays(fetch, relays)
		currentState.mutex.Lock()
		for _, event := range e {
			currentState.data[event.ID] = event
		}
		currentState.mutex.Unlock()
		persist()

		if len(missing) > 0 {
			k = false
			for _, s := range missing {
				fmt.Println("j87y8: " + s)
			}
			mindmachine.LogCLI("failed to get the events above, which are required to complete the eventPack", 2)
		}
	}

	var lastMessageTypeIsBlock bool = true
	var lastBlock = mindmachine.MakeOrGetConfig().GetInt64("ignitionHeight")
	for _, id := range eventPack {
		n, ok := currentState.data[id]
		if ok {
			events = append(events, mindmachine.ConvertToInternalEvent(&n))
			lastMessageTypeIsBlock = false
		}
		if !ok {
			if len(id) < 7 { //1000000!
				//if height is last+1 cool, if height is last+x then last message MUST be a block
				if height, err := cast.ToInt64E(id); err != nil {
					mindmachine.LogCLI(err, 1)
				} else {
					if height == lastBlock+1 || height > lastBlock && lastMessageTypeIsBlock {
						//cool
						lastMessageTypeIsBlock = true
						for lastBlock < height {
							lastBlock++
							b := makeBlock(lastBlock)
							events = append(events, mindmachine.ConvertToInternalEvent(&b))
						}
					} else {
						//cool story but wrong height
						if height != 761151 {
							mindmachine.LogCLI(fmt.Sprintf("invalid block in eventpack %d", height), 1)
						}
					}
				}
			} else {
				mindmachine.LogCLI("this should not happen", 1)
			}
		}
	}
	var missing []string
	var eventsIDs = make(map[string]struct{})
	for _, s := range events {
		eventsIDs[s.ID] = struct{}{}
		if s.Kind == 125 {
			if h, ok := s.GetSingleTag("height"); ok {
				eventsIDs[h] = struct{}{}
			}
		}
	}
	for _, s := range eventPack {
		if _, ok := eventsIDs[s]; !ok {
			missing = append(missing)
		}
	}
	if len(missing) == 0 {
		k = true
	} else {
		k = false
		for _, s := range missing {
			fmt.Println(s)
		}
		mindmachine.LogCLI("The above events are missing from the Eventpack", 1)
	}
	return
}

func PublishMissingEvents() {
	var allEvents = messagepack.GetRequired()
	fmt.Printf("\nTotal required events: %d", len(allEvents))

	for _, s := range append(mindmachine.MakeOrGetConfig().GetStringSlice("relaysMust"), mindmachine.MakeOrGetConfig().GetStringSlice("relaysOptional")...) {
		missing := FindMissingEvents(allEvents, s)
		if len(missing) > 0 {
			pool := nostr.NewRelayPool()
			errchan := pool.Add(s, nostr.SimplePolicy{Read: true, Write: true})
			go func() {
				for err := range errchan {
					e := fmt.Sprintf("d23k9y5: %s", err.Error())
					mindmachine.LogCLI(e, 2)
				}
			}()
			for _, s2 := range missing {
				currentState.mutex.Lock()
				event, ok := currentState.data[s2]
				if ok {
					fmt.Printf("\nRelay %s is missing event %s", s, s2)
					_, _, err := pool.PublishEvent(&event)
					if err != nil {
						mindmachine.LogCLI(err.Error(), 3)
						break
					}
					//time.Sleep(time.Second)
				} else {
					fmt.Println(51)
				}
				currentState.mutex.Unlock()
			}
		}

	}
}

func FindMissingEvents(eventPack []string, relay string) (missing []string) {
	_, missing, _ = fetchEventPack(eventPack, false, []string{relay})
	return missing
}

var fetchingMutex = &deadlock.Mutex{}

func fetchEventPack(eventPack []string, tryLocalFirst bool, relayList []string) (events []mindmachine.Event, missing []string, ok bool) {
	fetchingMutex.Lock()
	defer fetchingMutex.Unlock()
	//todo rebroadcast events to all relays
	defer persist()
	var temp = make(map[string]nostr.Event)
	var failed []string
	var fetch []string
	for _, s := range eventPack {
		if len(s) == 64 {
			fetch = append(fetch, s)
		}
	}
	var filters nostr.Filters
	var idsToSubscribe []string
	for _, s := range fetch {
		currentState.mutex.Lock()
		if localEvent, ok := currentState.data[s]; ok && tryLocalFirst {
			temp[s] = localEvent
		} else {
			idsToSubscribe = append(idsToSubscribe, s)
			//newFilter := nostr.Filter{
			//	IDs: []string{s},
			//}
			//filters = append(filters, newFilter)
		}
		currentState.mutex.Unlock()
	}
	if len(idsToSubscribe) > 0 {
		newFilter := nostr.Filter{
			IDs: idsToSubscribe,
		}
		filters = append(filters, newFilter)
	}
	if len(filters) > 0 {
		pool := nostr.NewRelayPool()
		var relays []string
		if len(relayList) > 0 {
			relays = append(relays, relayList...)
		} else {
			relays = mindmachine.MakeOrGetConfig().GetStringSlice("relaysMust")
		}
		for _, s := range relays {
			mindmachine.LogCLI(fmt.Sprintf("Connecting to relay %s", s), 3)
			errchan := pool.Add(s, nostr.SimplePolicy{Read: true, Write: true})
			go func() {
				for err := range errchan {
					e := fmt.Sprintf("k23459: %s", err.Error())
					mindmachine.LogCLI(e, 2)
				}
			}()
		}
		_, evts, unsub := pool.Sub(filters)
		attempts := 0
		gotResult := false
	E:
		for {
			select {
			case event := <-nostr.Unique(evts):
				if mindmachine.Contains(fetch, event.ID) {
					temp[event.ID] = event
					currentState.upsert(event)
					gotResult = true
				}
				continue
			case <-time.After((time.Second * 5) + (time.Second * time.Duration(len(eventPack)/20))):
				if gotResult {
					break E
				} else {
					fmt.Println(118)
					attempts++
					if attempts > 2 {
						break E
					}
					continue
				}
			}
		}
		unsub()
		//fmt.Printf("\n\n%#v\n", eventPack)
	}
	for _, s := range fetch {
		if _, ok := temp[s]; !ok {
			failed = append(failed, s)
		}
	}
	var lastMessageTypeIsBlock bool = true
	var lastBlock = mindmachine.MakeOrGetConfig().GetInt64("ignitionHeight")
	for _, id := range eventPack {
		n, ok := temp[id]
		if ok {
			events = append(events, mindmachine.ConvertToInternalEvent(&n))
			lastMessageTypeIsBlock = false
		}
		if !ok && len(id) < 7 { //1000000!
			//if height is last+1 cool, if height is last+x then last message MUST be a block
			if height, err := cast.ToInt64E(id); err != nil {
				mindmachine.LogCLI(err, 1)
			} else {
				if height == lastBlock+1 || height > lastBlock && lastMessageTypeIsBlock {
					//cool
					lastMessageTypeIsBlock = true
					for lastBlock < height {
						lastBlock++
						b := makeBlock(lastBlock)
						events = append(events, mindmachine.ConvertToInternalEvent(&b))
					}

				} else {
					//cool story
					if height != 761151 {
						mindmachine.LogCLI(fmt.Sprintf("invalid block in eventpack %d", height), 1)
					}
				}
			}

		}
	}
	if len(failed) > 0 {
		return events, failed, false
	}
	return events, []string{}, true
}

func makeBlock(h int64) nostr.Event {
	err := fmt.Errorf("")
	block := mindmachine.BlockHeader{}
	block.Height = h
	block.Hash = ""
	block.Time = time.Now().Unix()
	if !mindmachine.MakeOrGetConfig().GetBool("fastSync") {
		block, err = blocks.FetchBlock(h)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 0)
		}
	}
	b := nostr.Event{
		PubKey:    mindmachine.MyWallet().Account,
		CreatedAt: time.Now(),
		Kind:      125,
		Tags: nostr.Tags{
			[]string{"block", fmt.Sprintf("%d", block.Height), block.Hash, fmt.Sprintf("%d", block.Time)},
			[]string{"mind", "blocks"}},
	}
	err = b.Sign(mindmachine.MyWallet().PrivateKey)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	return b
}
