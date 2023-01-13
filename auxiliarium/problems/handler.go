package problems

import (
	"encoding/json"
	"fmt"

	"mindmachine/consensus/identity"
	"mindmachine/consensus/messagepack"
	"mindmachine/messaging/nostrelay"
	"mindmachine/mindmachine"
)

func HandleEvent(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	if mind, _ := mindmachine.WhichMindForKind(event.Kind); mind == "problems" {
		if identity.IsUSH(event.PubKey) {
			currentState.mutex.Lock()
			defer currentState.mutex.Unlock()
			switch event.Kind {
			case 640802:
				if handleUpdate(event) {
					return currentState.takeSnapshot(), true
				}
			case 640800:
				if handleNew(event) {
					return currentState.takeSnapshot(), true
				}
			}
			return
		}
	}
	return //for the IDE
}

func handleUpdate(event mindmachine.Event) bool {
	var unmarshalled Kind640802
	err := json.Unmarshal([]byte(event.Content), &unmarshalled)
	if err == nil {
		updates := 0
		targetItem, ok := currentState.data[unmarshalled.Target]
		if !ok {
			return false
		}
		if targetItem.Sequence+1 == unmarshalled.Sequence {
			if pubkeyHasCredentialsToUpdate(&event, &targetItem) {
				if len(unmarshalled.Parent) == 64 {
					if p, ok := currentState.data[unmarshalled.Parent]; ok {
						if p.canHaveChildren() {
							targetItem.Parent = unmarshalled.Parent
							updates++
						}
					}
				}
				if unmarshalled.RemoveParent {
					targetItem.Parent = ""
					updates++
				}
				if unmarshalled.RemoveClaim {
					targetItem.ClaimedBy = ""
					updates++
				}
				if len(unmarshalled.Title) == 64 {
					if t, ok := nostrelay.FetchEventPack([]string{unmarshalled.Title}); ok {
						messagepack.AddRequired(t[0].Nostr())
						targetItem.Title = unmarshalled.Title
						updates++
					}
				}
				if len(unmarshalled.Description) == 64 {
					if d, ok := nostrelay.FetchEventPack([]string{unmarshalled.Description}); ok {
						messagepack.AddRequired(d[0].Nostr())
						targetItem.Description = unmarshalled.Description
						updates++
					}
				}
				if unmarshalled.Close {
					if !targetItem.hasOpenChildren() {
						targetItem.Closed = true
						updates++
					}
				}
				if unmarshalled.ReOpen {
					targetItem.Closed = false
					updates++
				}
				if unmarshalled.Curate {
					targetItem.Curator = event.PubKey
					updates++
				}
			}
			if pubkeyHasCredentialsToClaim(&event, &targetItem) && targetItem.ClaimedBy == "" {
				if unmarshalled.Claim && !targetItem.hasOpenChildren() {
					targetItem.ClaimedBy = event.PubKey
					targetItem.ClaimedAt = mindmachine.CurrentState().Processing.Height
					updates++
				}
			}
			if targetItem.ClaimedBy == event.PubKey {
				if unmarshalled.RemoveClaim {
					targetItem.ClaimedBy = ""
					targetItem.ClaimedAt = 0
					updates++
				}
			}
			if updates > 0 {
				targetItem.Sequence++
				targetItem.LastUpdate = mindmachine.CurrentState().Processing.Height
				currentState.upsert(targetItem)
				return true
			}
		} else {
			mindmachine.LogCLI(fmt.Sprintf("Invalid sequence on event %s (%d) target item has sequence of %d", event.ID, unmarshalled.Sequence, targetItem.Sequence), 4)
			return false
		}
	} else if err != nil {
		mindmachine.LogCLI(err.Error(), 1)
	}
	return false
}

func (item *Problem) canHaveChildren() bool {
	if item.Closed {
		return false
	}
	if len(item.ClaimedBy) > 0 {
		return false
	}
	return true
}

func pubkeyHasCredentialsToClaim(e *mindmachine.Event, target *Problem) bool {
	if pubkeyHasCredentialsToUpdate(e, target) {
		return true
	}
	if identity.IsUSH(e.PubKey) {
		return true
	}
	return false
}

func pubkeyHasCredentialsToUpdate(e *mindmachine.Event, target *Problem) bool {
	if identity.IsMaintainer(e.PubKey) {
		return true
	}
	if target.CreatedBy == e.PubKey {
		return true
	}
	if target.Curator == e.PubKey {
		return true
	}
	return false
}

func handleNew(event mindmachine.Event) bool {
	if _, exists := currentState.data[mindmachine.Sha256(event.ID)]; !exists {
		var unmarshalled Kind640800
		err := json.Unmarshal([]byte(event.Content), &unmarshalled)
		if err == nil {
			item := Problem{
				UID:         mindmachine.Sha256(event.ID),
				CreatedBy:   event.PubKey,
				WitnessedAt: mindmachine.CurrentState().Processing.Height,
				Sequence:    1,
				LastUpdate:  mindmachine.CurrentState().Processing.Height,
				Closed:      false,
				ClaimedBy:   "",
				Curator:     event.PubKey,
				ClaimedAt:   0,
			}
			if len(unmarshalled.Title) == 64 {
				var et, ed mindmachine.Event
				if t, ok := nostrelay.FetchEventPack([]string{unmarshalled.Title}); ok {
					et = t[0]
					item.Title = unmarshalled.Title
					if len(unmarshalled.Description) == 64 {
						if d, ok := nostrelay.FetchEventPack([]string{unmarshalled.Description}); ok {
							ed = d[0]
							item.Description = unmarshalled.Description
						}
					}
					if len(unmarshalled.Parent) == 64 {
						parent, ok := currentState.data[unmarshalled.Parent]
						if ok {
							if parent.canHaveChildren() {
								item.Parent = unmarshalled.Parent
								currentState.upsert(item)
								messagepack.AddRequired(et.Nostr())
								messagepack.AddRequired(ed.Nostr())
								return true
							}
						} else {
							//15171031
							if event.ID == "3d666acf9483baed64213e79aad72f1db808a903ccd5e2c0fb48388fe6eb89e6" {
								item.Parent = unmarshalled.Parent
								currentState.upsert(item)
								return true
							}
						}
					}
				}

			}
		} else {
			mindmachine.LogCLI(err, 1)
		}
	}
	return false
}
