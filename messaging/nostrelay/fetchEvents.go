package nostrelay

import (
	"fmt"
	"time"

	"github.com/fiatjaf/go-nostr"
	"github.com/spf13/cast"
	"mindmachine/messaging/blocks"
	"mindmachine/mindmachine"
)

var relayPool *nostr.RelayPool
var sk string

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
	if len(filters) > 0 {
		relays := mindmachine.MakeOrGetConfig().GetStringSlice("relays")
		pool := initRelays(relays)
		fmt.Printf("\nFetching Events:\n%#v\n", idsToSubscribe)
		sub := pool.Sub(filters)
		attempts := 0
		gotResult := false
		for {
			select {
			case event := <-sub.UniqueEvents:
				if mindmachine.Contains(fetch, event.ID) {
					temp[event.ID] = event
					currentState.upsert(event)
					gotResult = true
				}
				continue
			case <-time.After(1 * time.Second):
				if gotResult {
					break
				}
				fmt.Println(54)
				attempts++
				if attempts > len(idsToSubscribe) {
					break
				} else {
					continue
				}
			}
			break
		}
		sub.Unsub()
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
					mindmachine.LogCLI(fmt.Sprintf("invalid block in eventpack %d", height), 1)
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

//func proxySubscription(filter nostr.Filters, response chan nostr.Event) {
//	defer persist()
//	relays := mindmachine.MakeOrGetConfig().GetStringSlice("relays")
//	pool := initRelays(relays)
//	sub := pool.Sub(filter)
//	fmt.Printf("\n136\n%#v\n", filter)
//	for {
//		select {
//		case event := <-sub.UniqueEvents:
//			go func() {
//				response <- event
//			}()
//			currentState.upsert(event)
//			continue
//		case <-time.After(5 * time.Second):
//			break
//		}
//		break
//	}
//	sub.Unsub()
//}

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
			nostr.StringList{"block", fmt.Sprintf("%d", block.Height), block.Hash, fmt.Sprintf("%d", block.Time)},
			nostr.StringList{"mind", "blocks"}},
	}
	err = b.Sign(mindmachine.MyWallet().PrivateKey)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	return b
}