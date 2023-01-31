package nostrkinds

import (
	"encoding/json"

	"mindmachine/consensus/identity"
	"mindmachine/mindmachine"
)

func HandleEvent(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	switch event.Kind {
	case 641800:
		if handle641800(event) {
			writeDb()
			return h, true
		}
	}
	return
}

func handle641800(event mindmachine.Event) bool {
	var unmarshalled Kind641800
	err := json.Unmarshal([]byte(event.Content), &unmarshalled)
	if err == nil {
		existing, ok := currentState.data[unmarshalled.Kind]
		if ok {
			if unmarshalled.Sequence == existing.Sequence+1 {
				if event.PubKey == existing.Curator || identity.IsMaintainer(event.PubKey) {
					if updated, updates := update(&unmarshalled, existing); updates > 0 {
						currentState.upsert(updated)
						return true
					}
				}
				return false
			}
		}
		if !ok {
			if identity.IsUSH(event.PubKey) && unmarshalled.Sequence == 1 {
				if updated, updates := update(&unmarshalled, existing); updates > 0 {
					if len(existing.Curator) == 0 {
						updated.Curator = event.PubKey
					}
					currentState.upsert(updated)
					return true
				}
			}
			return false
		}
	}
	if err != nil {
		mindmachine.LogCLI(err.Error(), 3)
	}
	return false
}

func update(unmarshalled *Kind641800, existing Kind) (Kind, int64) {
	var updates int64
	existing.Kind = unmarshalled.Kind
	if unmarshalled.RemoveCurator {
		existing.Curator = ""
		updates++
	}
	if len(unmarshalled.Description) > 0 && unmarshalled.Description != existing.Description {
		existing.Description = unmarshalled.Description
		updates++
	}
	if len(unmarshalled.App) > 0 && unmarshalled.App != existing.App {
		existing.App = unmarshalled.App
		updates++
	}

	if !mindmachine.Contains(existing.NIPs, unmarshalled.NIP) && len(unmarshalled.NIP) > 0 {
		existing.NIPs = append(existing.NIPs, unmarshalled.NIP)
		updates++
	}
	if updates > 0 {
		existing.Sequence++
	}
	return existing, updates
}
