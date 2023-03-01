package mindmachine

import (
	"fmt"
	"strings"
	"time"

	"github.com/sasha-s/go-deadlock"
	"github.com/stackerstan/go-nostr"
)

var mutex = &deadlock.Mutex{}

func PruneDeadOptionalRelays() {
	mutex.Lock()
	defer mutex.Unlock()
	pruneDeadRelays()
}

var lastPrune time.Time

func pruneDeadRelays() {
	if time.Since(lastPrune) > (time.Second * 60 * 5) {
		lastPrune = time.Now()
		relays := MakeOrGetConfig().GetStringSlice("relaysOptional")
		if len(relays) == 0 {
			if optionalRelays, ok := getOptionalRelays(); ok {
				relays = optionalRelays
			}

		}
		newRelays := prune(relays)
		MakeOrGetConfig().SetDefault("relaysOptional", newRelays)
		MakeOrGetConfig().Set("relaysOptional", newRelays)
		//MakeOrGetConfig().WriteConfigAs("newconfig.yaml")
		if err := MakeOrGetConfig().WriteConfig(); err != nil {
			LogCLI(err.Error(), 2)
		}
		LogCLI(fmt.Sprintf("Pruned %d optional relays, we now have %d optional relays in the config file.", len(relays)-len(newRelays), len(newRelays)), 3)
	}
}

func GetSomeRelays() ([]string, bool) {
	return getOptionalRelays()
}

func Prune(input []string) (output []string) {
	return prune(input)
}
func prune(input []string) (output []string) {
	//fmt.Println(input)
	wait := deadlock.WaitGroup{}
	failedRelays := make(chan string, len(input))
	var failedRelayMap = make(map[string]struct{})
	var processing int64
	for _, s := range input {
		//for processing >= 10 {
		//	//limit the number of relays we test concurrently
		//	<-time.After(time.Millisecond * 500)
		//}
		wait.Add(1)
		processing++
		go func(relay string) {
			pool := nostr.NewRelayPool()
			errchan := pool.Add(relay, nostr.SimplePolicy{Read: true, Write: true})
			failed := make(chan struct{})
			go func(failed chan struct{}) {
				for err := range errchan {
					if strings.Contains(err.Error(), "failed") {
						close(failed)
					}
				}
			}(failed)
			filters := nostr.Filters{}
			filters = append(filters, nostr.Filter{
				//Kinds: []int{640001},
			})
			_, evnts, unsub := pool.Sub(filters)
		L:
			for {
				select {
				case <-nostr.Unique(evnts):
					break L
				case <-time.After(time.Second * 4):
					failedRelays <- relay
					break L
				case <-failed:
					failedRelays <- relay
					break L
				}
			}
			//fmt.Println("relays.go:74")
			unsub()
			pool.Remove(relay)
			processing--
			wait.Done()
		}(s)
	}
	//fmt.Println("relays.go:79")
	wait.Wait()
	for {
		select {
		case failed := <-failedRelays:
			failedRelayMap[failed] = struct{}{}
		case <-time.After(time.Second):
			for _, s := range input {
				if _, present := failedRelayMap[s]; !present {
					output = append(output, s)
				}
			}
			return
		}
	}
}
