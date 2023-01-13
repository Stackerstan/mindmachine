package messagepack

import (
	"github.com/sasha-s/go-deadlock"
	"github.com/stackerstan/go-nostr"
)

var RequiredEvents = make(map[string]nostr.Event)
var reMutex = &deadlock.Mutex{}

func AddRequired(e nostr.Event) {
	reMutex.Lock()
	defer reMutex.Unlock()
	RequiredEvents[e.ID] = e
}

func GetRequired() (e []string) {
	reMutex.Lock()
	defer reMutex.Unlock()
	for _, event := range RequiredEvents {
		e = append(e, event.ID)
	}
	return
}
