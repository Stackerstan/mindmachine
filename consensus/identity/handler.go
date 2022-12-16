package identity

import (
	"encoding/json"

	"mindmachine/mindmachine"
)

//todo problem we are not adding account to Identity

func HandleEvent(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	if sig, _ := event.CheckSignature(); !sig {
		return
	}
	if mind, _ := mindmachine.WhichMindForKind(event.Kind); mind == "identity" {
		currentState.mutex.Lock()
		defer currentState.mutex.Unlock()
		var updates int64 = 0
		existingIdentity := getLatestIdentity(event.PubKey)
		var updateIdents []Identity
		//if event.Sequence() == existingIdentity.Sequence+1 {
		if event.Kind == 640400 {
			var unmarshalled Kind640400
			err := json.Unmarshal([]byte(event.Content), &unmarshalled)
			if err != nil {
				mindmachine.LogCLI(err.Error(), 3)
				return
			} else {
				if unmarshalled.Sequence == existingIdentity.Sequence+1 {
					if existingIdentity.Sequence == 0 {
						if !nameTaken(unmarshalled.Name) {
							//we don't care if a name has been set, it's not permanent if sequence is 0
							existingIdentity.Name = unmarshalled.Name
							updates++
						}
					}
					if len(unmarshalled.Name) > 0 && len(unmarshalled.Name) <= 20 {
						//sequence is > 0, we can't change the name if it's already been set
						if existingIdentity.addName(unmarshalled.Name) {
							updates++
						}
					}
					if len(unmarshalled.About) > 0 && len(unmarshalled.About) <= 560 {
						if existingIdentity.upsertBio(unmarshalled.About) {
							updates++
						}
					}
				}
			}
		}
		if event.Kind == 640402 {
			var unmarshalled Kind640402
			err := json.Unmarshal([]byte(event.Content), &unmarshalled)
			if err != nil {
				mindmachine.LogCLI(err.Error(), 3)
				return
			} else if unmarshalled.Sequence == existingIdentity.Sequence+1 {
				target, okt := currentState.data[unmarshalled.Target]
				bestower, okb := currentState.data[event.PubKey]
				var updts int64
				if okt && okb && len(target.Name) > 0 {
					if unmarshalled.Maintainer {
						if len(bestower.MaintainerBy) > 0 {
							if len(target.MaintainerBy) == 0 {
								target.MaintainerBy = event.PubKey
								updts++
							}
						}
					}
					if unmarshalled.Character {
						if _, exists := target.CharacterVouchedForBy[event.PubKey]; !exists {
							target.CharacterVouchedForBy[event.PubKey] = struct{}{}
							updts++
						}
					}
					if unmarshalled.USH {
						if len(target.UniqueSovereignBy) == 0 {
							var order int64
							for _, identity := range currentState.data {
								if identity.Order > order {
									order = identity.Order
								}
							}
							target.UniqueSovereignBy = event.PubKey
							target.Order = order + 1
							updts++
						}
					}
					if updts > 0 {
						updateIdents = append(updateIdents, target)
						updates++
					}
				}
			}
		}
		if event.Kind == 640404 {
			var unmarshalled Kind640404
			err := json.Unmarshal([]byte(event.Content), &unmarshalled)
			if err != nil {
				mindmachine.LogCLI(err.Error(), 3)
				return
			} else if unmarshalled.Sequence == existingIdentity.Sequence+1 {
				//todo length validation
				var zucker LegacySocialMediaIdentity
				zucker.EvidenceURL = unmarshalled.Evidence
				zucker.Platform = unmarshalled.Platform
				zucker.Username = unmarshalled.Username
				existingIdentity.LegacyIdentities = append(existingIdentity.LegacyIdentities, zucker)
				updates++
			}
		}
		if event.Kind == 640406 {
			var unmarshalled Kind640406
			err := json.Unmarshal([]byte(event.Content), &unmarshalled)
			if err != nil {
				mindmachine.LogCLI(err.Error(), 3)
				return
			} else if unmarshalled.Sequence == existingIdentity.Sequence+1 {
				existingIdentity.OpReturnAddr = append(existingIdentity.OpReturnAddr, []string{unmarshalled.Address, unmarshalled.Proof})
				updates++
			}
		}
		if updates > 0 {
			existingIdentity.Sequence++
			existingIdentity.Account = event.PubKey
			currentState.data[event.PubKey] = existingIdentity
			for _, ident := range updateIdents {
				ident.Sequence++
				currentState.data[ident.Account] = ident
			}
			return currentState.takeSnapshot(), true
		}
	}
	return
}

func nameTaken(name string) bool {
	var taken bool
	for _, identity := range currentState.data {
		if identity.Name == name {
			taken = true
		}
	}
	return taken
}

func (i *Identity) addName(content string) bool {
	if nameTaken(content) {
		return false
	}
	if len(i.Name) > 0 {
		return false
	}
	if len(content) > 30 {
		return false
	}
	i.Name = content
	return true
}

func (i *Identity) upsertBio(content string) bool {
	//todo use nostr bio message
	i.About = content
	return true
}

func (i *Identity) addLegacyIdentity(platform, username, evidence string) bool {
	var gtg int64
	if len(platform) > 3 && len(platform) < 20 {
		gtg++
	}
	if len(username) > 3 && len(username) < 30 {
		gtg++
	}
	if len(evidence) > 3 && len(evidence) < 20 {
		gtg++
	}
	if gtg == 3 {
		i.LegacyIdentities = append(i.LegacyIdentities, LegacySocialMediaIdentity{
			Platform:    platform,
			Username:    username,
			EvidenceURL: evidence,
		})
		return true
	}
	return false
}

func (i *Identity) name() {

}
