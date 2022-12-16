package doki

import (
	"encoding/json"

	"github.com/sergi/go-diff/diffmatchpatch"
	"mindmachine/consensus/identity"
	"mindmachine/mindmachine"
)

func HandleEvent(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	if mind, _ := mindmachine.WhichMindForKind(event.Kind); mind == "doki" {
		if identity.IsUSH(event.PubKey) {
			currentState.mutex.Lock()
			defer currentState.mutex.Unlock()
			switch event.Kind {
			case 641200:
				return handle641200(event)
			case 641202:
				return handle641202(event)
			case 641204:
				return handle641204(event)
			}
		}
	}
	return
}

func handle641204(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	var unmarshalled Kind641204
	err := json.Unmarshal([]byte(event.Content), &unmarshalled)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 3)
	} else {
		if identity.IsMaintainer(event.PubKey) {
			if existingDocument, ok := currentState.data[unmarshalled.DocumentUID]; ok {
				if existingDocument.Sequence+1 == unmarshalled.Sequence {
					for i, patch := range existingDocument.Patches {
						if len(patch.MergedBy) != 64 && len(patch.RejectedBy) != 64 {
							if unmarshalled.PatchEventID == patch.EventID {
								switch unmarshalled.Operation {
								case 1:
									existingDocument.Patches[i].RejectedBy = event.PubKey
									existingDocument.Patches[i].RejectedReason = unmarshalled.Reason
								case 2:
									if patch.CreatedBy == event.PubKey && event.PubKey != mindmachine.IgnitionAccount {
										//we are not allowed to merge our own patches
										return
									}
									dmp := diffmatchpatch.New()
									p, err := dmp.PatchFromText(patch.Patch)
									if err != nil {
										mindmachine.LogCLI(err.Error(), 1)
										return
									}
									newTip, _ := dmp.PatchApply(p, existingDocument.CurrentTip)
									existingDocument.CurrentTip = newTip
									existingDocument.Patches[i].MergedBy = event.PubKey
								}
								existingDocument.Sequence++
								if len(existingDocument.MergedBy) != 64 {
									existingDocument.MergedBy = event.PubKey
								}
								currentState.data[unmarshalled.DocumentUID] = existingDocument
								return currentState.takeSnapshot(), true
							}
						}
					}
				}
			}
		}
	}
	return
}

func handle641202(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	var unmarshalled Kind641202
	err := json.Unmarshal([]byte(event.Content), &unmarshalled)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 3)
	} else {
		if identity.IsUSH(event.PubKey) {
			if existingDocument, ok := currentState.data[unmarshalled.DocumentUID]; ok {
				if existingDocument.Sequence+1 == unmarshalled.Sequence {
					existingDocument.Patches = append(existingDocument.Patches, Patch{
						EventID:   event.ID,
						Patch:     unmarshalled.Patch,
						CreatedBy: event.PubKey,
						Problem:   unmarshalled.Problem,
					})
					existingDocument.Sequence++
					currentState.data[unmarshalled.DocumentUID] = existingDocument
					return currentState.takeSnapshot(), true

				}
			}
		}
	}
	return
}

func handle641200(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	var unmarshalled Kind641200
	err := json.Unmarshal([]byte(event.Content), &unmarshalled)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 3)
	} else {
		if identity.IsUSH(event.PubKey) {
			uid := mindmachine.Sha256(unmarshalled.GoalOrProblem)
			if _, exists := currentState.data[uid]; !exists {
				if len(unmarshalled.GoalOrProblem) > 30 {
					var newdoc Document
					newdoc.UID = uid
					newdoc.GoalOrProblem = unmarshalled.GoalOrProblem
					newdoc.CreatedBy = event.PubKey
					newdoc.Sequence = 1
					currentState.data[newdoc.UID] = newdoc
					return currentState.takeSnapshot(), true
				}
			}
		}
	}
	return
}
