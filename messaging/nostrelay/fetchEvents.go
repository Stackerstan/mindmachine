package nostrelay

import (
	"fmt"
	"time"

	"github.com/spf13/cast"
	"github.com/stackerstan/go-nostr"
	"mindmachine/messaging/blocks"
	"mindmachine/mindmachine"
)

func FetchLocalCachedEvent(event string) (nostr.Event, bool) {
	if localEvent, ok := currentState.data[event]; ok {
		return localEvent, true
	}
	return nostr.Event{}, false
}

func FetchEventPack(eventPack []string) (events []mindmachine.Event, ok bool) {
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
		if localEvent, ok := currentState.data[s]; ok {
			temp[s] = localEvent
		} else {
			idsToSubscribe = append(idsToSubscribe, s)
			//newFilter := nostr.Filter{
			//	IDs: []string{s},
			//}
			//filters = append(filters, newFilter)
		}
	}
	if len(idsToSubscribe) > 0 {
		newFilter := nostr.Filter{
			IDs: idsToSubscribe,
		}
		filters = append(filters, newFilter)
	}
	//fmt.Printf("\n\n%#v\n", filters)
	if len(filters) > 0 {
		pool := nostr.NewRelayPool()
		mindmachine.LogCLI("Connecting to relay pool", 3)
		for _, s := range mindmachine.MakeOrGetConfig().GetStringSlice("relays") {
			errchan := pool.Add(s, nostr.SimplePolicy{Read: true, Write: true})
			go func() {
				for err := range errchan {
					mindmachine.LogCLI(err.Error(), 2)
				}
			}()
		}
		defer func() {
			for _, s := range mindmachine.MakeOrGetConfig().GetStringSlice("relays") {
				pool.Remove(s)
			}
		}()
		//errchan := pool.Add("wss://nostr.688.org/", nostr.SimplePolicy{Read: true, Write: true})
		//go func() {
		//	for err := range errchan {
		//		fmt.Println(err.Error())
		//	}
		//}()

		//relays := mindmachine.MakeOrGetConfig().GetStringSlice("relays")
		//pool := initRelays(relays)
		//pool := nostr.NewRelayPool()
		//
		//_ = pool.Add("wss://nostr.688.org/", nostr.SimplePolicy{Read: true, Write: true})
		fmt.Printf("\nFetching Events:\n%#v\n", idsToSubscribe)
		_, evts, unsub := pool.Sub(filters)
		attempts := 0
		gotResult := false
		for {
			select {
			case event := <-nostr.Unique(evts):
				if mindmachine.Contains(fetch, event.ID) {
					temp[event.ID] = event
					currentState.upsert(event)
					gotResult = true
				}
				continue
			case <-time.After(3 * time.Second):
				if gotResult {
					fmt.Println(116)
					break
				}
				fmt.Println(118)
				attempts++
				if attempts > len(idsToSubscribe) {
					break
				} else {
					continue
				}
			}
			break
		}
		unsub()
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
	//if len(events) != len(eventPack) {
	//	mindmachine.LogCLI("this should not happen", 0)
	//}
	if len(failed) > 0 {
		for _, s := range failed {
			fmt.Println("nostrelay:65 " + s)
		}
		mindmachine.LogCLI("failed to get the events above, which are required to complete the eventPack", 2)
		return events, false
	}
	return events, true
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
