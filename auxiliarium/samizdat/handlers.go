package samizdat

import (
	"mindmachine/messaging/nostrelay"
	"mindmachine/mindmachine"
)

func HandleEvent(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	if mind, _ := mindmachine.WhichMindForKind(event.Kind); mind == "samizdat" {
		currentState.mutex.Lock()
		defer currentState.mutex.Unlock()
		switch event.Kind {
		case 1:
			return handle1(event)
		case 5:
		}
	}
	return
}

func handle1(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	if _, exists := currentState.data[event.ID]; !exists {
		if t, ok := event.GetTags("e"); ok && len(event.Content) < 561 {
			if t[0] == currentState.findRoot() {
				if saz, k := currentState.data[t[len(t)-1]]; k {
					saz.Children = append(saz.Children, event.ID)
					currentState.data[t[len(t)-1]] = saz
					currentState.data[event.ID] = Samizdat{
						ID:       event.ID,
						Parent:   t[len(t)-1],
						Children: []string{},
					}
					nostrelay.CacheEventLocally(event.Nostr())
					return currentState.takeSnapshot(), true
				}
			}
		} else if currentState.findRoot() == "0" && event.PubKey == mindmachine.IgnitionAccount {
			if event.ID == "9e333343184fe3e98b028782f7098cf596f1f46adf546541e7317d9a5f1d5d57" {
				currentState.data[event.ID] = Samizdat{
					ID:       event.ID,
					Parent:   "",
					Children: []string{},
				}
				return currentState.takeSnapshot(), true
			}
		}
	}
	return
}

func handle5(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	return
}
