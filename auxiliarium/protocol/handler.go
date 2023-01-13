package protocol

//todo validate sequence numbers
import (
	"encoding/json"
	"fmt"

	"mindmachine/consensus/identity"
	"mindmachine/consensus/messagepack"
	"mindmachine/consensus/shares"
	"mindmachine/messaging/nostrelay"
	"mindmachine/mindmachine"
)

func HandleEvent(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	if mind, _ := mindmachine.WhichMindForKind(event.Kind); mind == "protocol" {
		if identity.IsUSH(event.PubKey) {
			currentState.mutex.Lock()
			defer currentState.mutex.Unlock()
			switch event.Kind {
			case 640600:
				if handleNew(event) {
					return currentState.takeSnapshot(), true
				}
			case 640602:
				if handleVote(event) {
					return currentState.takeSnapshot(), true
				}
			case 640604:
				if handleReorderNests(event) {
					return currentState.takeSnapshot(), true
				}
			default:
				return
			}
		}
	}
	return
}

func handleReorderNests(event mindmachine.Event) (b bool) {
	//problem: it's inconvenient to reorder nested items to improve readability
	//solution: allow any maintainer or votepowered participant to reorder nests
	if identity.IsMaintainer(event.PubKey) || shares.VotePowerForAccount(event.PubKey) > 0 {
		var unmarshalled Kind640604
		if err := json.Unmarshal([]byte(event.Content), &unmarshalled); err == nil {
			if currentItem, ok := currentState.data[unmarshalled.Target]; ok {
				//validate that the participant hasn't changed, added, or removed any UIDs
				currentNests := make(map[string]int64)
				proposedNests := make(map[string]int64)
				for _, nest := range currentItem.Nests {
					currentNests[nest] = 0
				}
				for _, nest := range unmarshalled.Nests {
					proposedNests[nest] = 0
				}
				for s, i := range currentNests {
					if p, ok := proposedNests[s]; ok {
						if p != 0 {
							return
						}
						proposedNests[s] = 1
						if i != 0 {
							return
						}
						currentNests[s] = 1
					}
				}
				for _, i := range currentNests {
					if i != 1 {
						return
					}
				}
				for _, i := range proposedNests {
					if i != 1 {
						return
					}
				}
				currentItem.Nests = unmarshalled.Nests
				currentState.data[unmarshalled.Target] = currentItem
				return true
			}
		} else if err != nil {
			mindmachine.LogCLI(err.Error(), 1)
		}
	}
	return
}

func handleVote(event mindmachine.Event) (b bool) {
	var unmarshalled Kind640602
	err := json.Unmarshal([]byte(event.Content), &unmarshalled)
	if err == nil {
		updates := 0
		targetItem, ok := currentState.data[unmarshalled.Target]
		if !ok {
			return
		}
		if targetItem.Blackballers == nil {
			targetItem.Blackballers = make(map[mindmachine.Account]struct{})
		}
		if targetItem.Ratifiers == nil {
			targetItem.Ratifiers = make(map[mindmachine.Account]struct{})
		}
		if targetItem.Sequence+1 != unmarshalled.Sequence {
			return
		}
		if shares.VotePowerForAccount(event.PubKey) > 0 {
			if unmarshalled.Ratify && unmarshalled.Blackball {
				return
			}
			for account, _ := range targetItem.Ratifiers {
				if event.PubKey == account {
					return
				}
			}
			for account, _ := range targetItem.Blackballers {
				if event.PubKey == account {
					return
				}
			}
			if targetItem.ApprovedAt == 0 && unmarshalled.Ratify {
				targetItem.Ratifiers[event.PubKey] = struct{}{}
				updates++
			}

			if targetItem.ApprovedAt == 0 && unmarshalled.Blackball {
				targetItem.Blackballers[event.PubKey] = struct{}{}
				updates++
			}
		}
		//todo add in the blackball:ratify per height ratios from the protocol
		if permille := shares.Permille(targetItem.Ratifiers); permille == 1000 {
			targetItem.ApprovedAt = mindmachine.CurrentState().Processing.Height
			updates++
		}
		if updates > 0 {
			targetItem.Sequence++
			currentState.data[unmarshalled.Target] = targetItem
			if targetItem.ApprovedAt > 0 && len(targetItem.Supersedes) == 64 {
				superseded, sok := currentState.data[targetItem.Supersedes]
				if sok {
					if len(superseded.SupersededBy) == 0 && superseded.Kind == targetItem.Kind {
						superseded.SupersededBy = unmarshalled.Target
						superseded.Sequence++
						superseded.LastUpdate = mindmachine.CurrentState().Processing.Height
						currentState.data[superseded.UID] = superseded
					}
				}
			}
			return true
		}
		return
	} else if err != nil {
		mindmachine.LogCLI(err.Error(), 1)
	}
	return false

}

func handleNew(event mindmachine.Event) (b bool) {
	var unmarshalled Kind640600
	err := json.Unmarshal([]byte(event.Content), &unmarshalled)
	if err == nil {
		item := Item{
			UID:          mindmachine.Sha256(event.ID),
			CreatedBy:    event.PubKey,
			WitnessedAt:  mindmachine.CurrentState().Processing.Height,
			Sequence:     1,
			LastUpdate:   mindmachine.CurrentState().Processing.Height,
			Ratifiers:    make(map[mindmachine.Account]struct{}),
			Blackballers: make(map[mindmachine.Account]struct{}),
			ApprovedAt:   0,
			SupersededBy: "",
		}
		valid := 0
		if len(unmarshalled.Problem) == 64 {
			//todo return false if we don't have the problem or it isn't open
			item.Problem = unmarshalled.Problem
			valid++
		}
		if len(unmarshalled.Text) == 64 {
			//todo validate text is Latin and has no markdown etc.
			if textEvent, ok := nostrelay.FetchEventPack([]string{unmarshalled.Text}); ok {
				if len(textEvent[0].Content) > 0 && len(textEvent[0].Content) <= 560 {
					messagepack.AddRequired(textEvent[0].Nostr())
					item.Text = unmarshalled.Text
					valid++
				}
			}
		}

		if len(unmarshalled.Kind) > 0 {
			var ival int64 = 0
			switch unmarshalled.Kind {
			case "definition":
				ival = Definition
			case "goal":
				ival = Goal
			case "rule":
				ival = Rule
			case "invariant":
				ival = Invariant
			case "protocol":
				ival = Protocol
			default:
				return
			}
			item.Kind = ival
			valid++
		}
		if len(unmarshalled.Parent) == 64 {
			if _, ok := currentState.data[unmarshalled.Parent]; ok {
				item.Parent = unmarshalled.Parent
				valid++
			} else if len(currentState.data) == 0 {
				//This is the root of the SSP
				valid++
			}
		}
		if valid != 4 {
			return
		}
		var superseder bool
		if len(unmarshalled.Supersedes) == 64 {
			if super, ok := currentState.data[unmarshalled.Supersedes]; !ok {
				return
			} else {
				if super.Kind != item.Kind {
					return
				}
			}
			item.Supersedes = unmarshalled.Supersedes
			superseder = true
		}
		if len(unmarshalled.Nests) > 0 {
			for _, s := range unmarshalled.Nests {
				//todo verify every nest is valid item
				//todo verify that the nested item lists this as a parent
				if len(s) == 64 {
					item.Nests = append(item.Nests, s)
				}
			}
		}
		if vp := shares.VotePowerForAccount(event.PubKey); vp > 0 {
			item.Ratifiers[event.PubKey] = struct{}{}
		}

		if permille := shares.Permille(item.Ratifiers); permille == 1000 {
			item.ApprovedAt = mindmachine.CurrentState().Processing.Height
		}
		currentState.upsert(item)
		if item.ApprovedAt > 0 {
			if superseder {
				err := currentState.updateSupersedes(item.Supersedes, item.UID)
				if err != nil {
					mindmachine.LogCLI(err.Error(), 1)
				}
				err = currentState.updateNests(item.Supersedes, item.UID)
				if err != nil {
					mindmachine.LogCLI(err.Error(), 1)
				}
			}
			//add to parent's nests if approved
			if len(item.Parent) == 64 {
				if parent, ok := currentState.data[item.Parent]; ok {
					parent.Nests = append(parent.Nests, item.UID)
					currentState.upsert(parent)
				}
			}

		}
		return true
	} else if err != nil {
		mindmachine.LogCLI(err.Error(), 1)
		return false
	}
	return false
}

func (d *db) updateSupersedes(original, supersededBy string) error {
	if item, ok := d.data[original]; ok {
		if _, ok := d.data[supersededBy]; ok {
			item.SupersededBy = supersededBy
			d.upsert(item)
			return nil
		}
		return fmt.Errorf(supersededBy, " does not exist")
	}
	return fmt.Errorf(original, " does not exist")
}

func (d *db) updateNests(original, supersededBy string) error {
	fmt.Println(175)
	if _, ok := d.data[supersededBy]; ok {
		if _, ok := d.data[original]; ok {
			for _, item := range d.data {
				for i, nest := range item.Nests {
					if nest == original {
						item.Nests[i] = supersededBy
					}
				}
				d.upsert(item)
			}
			return nil
		}
		return fmt.Errorf(original, " does not exist")
	}
	return fmt.Errorf(supersededBy, " does not exist")
}
