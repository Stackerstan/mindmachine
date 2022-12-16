package patches

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"mindmachine/consensus/identity"
	"mindmachine/messaging/nostrelay"
	"mindmachine/mindmachine"
)

func HandleEvent(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	if sig, _ := event.CheckSignature(); !sig {
		return
	}
	if mind, _ := mindmachine.WhichMindForKind(event.Kind); mind == "patches" {
		if identity.IsUSH(event.PubKey) {
			currentState.mutex.Lock()
			defer currentState.mutex.Unlock()
			switch event.Kind {
			case 641000:
				return handleNewRepo(event)
			case 641002:
				return handleNewPatch(event)
			case 641004:
				return handleMergePatch(event)
			}
		}
	}
	return
}

func handleMergePatch(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	if identity.IsMaintainer(event.PubKey) {
		var unmarshalled Kind641004
		err := json.Unmarshal([]byte(event.Content), &unmarshalled)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 1)
		}
		if err == nil {
			if repo, ok := getRepo(unmarshalled.RepoName); ok {
				if offer, ok := repo.Data[unmarshalled.UID]; ok {
					if unmarshalled.Conflicts && !offer.Conflicts {
						offer.Conflicts = true
						hs, err := repo.upsert(offer)
						if err != nil {
							mindmachine.LogCLI(err.Error(), 1)
							return h, false
						}
						return hs, true
					}
					tip := repo.getLatestPatch()
					if event.PubKey != offer.CreatedBy {
						if unmarshalled.Sequence == offer.Sequence+1 { // && unmarshalled.Height == tip.Height+1 {
							err := repo.validateNoConflicts(offer)
							if err != nil {
								mindmachine.LogCLI(err.Error(), 1)
								return h, false
							} else {
								offer.Maintainer = event.PubKey
								offer.Sequence = unmarshalled.Sequence
								offer.Height = tip.Height + 1
								hs, err := repo.upsert(offer)
								if err != nil {
									mindmachine.LogCLI(err.Error(), 1)
									return h, false
								}
								return hs, true
							}
						}
					}

				}
			}
		}
	}
	return
}

func handleNewPatch(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	var unmarshalled Kind641002
	err := json.Unmarshal([]byte(event.Content), &unmarshalled)
	if err != nil {
		mindmachine.LogCLI("failed to unmarshal event", 1)
		return h, false
	}
	if len(unmarshalled.Shards) > 0 {
		rebuilt, ok := rebuildFromShards(&unmarshalled)
		if !ok {
			mindmachine.LogCLI("failed to rebuild patch", 1)
			return
		}
		unmarshalled = rebuilt
	}
	if repo, ok := currentState.data[unmarshalled.RepoName]; ok {
		bs, err := hex.DecodeString(fmt.Sprintf("%s", unmarshalled.Diff))
		if err != nil {
			mindmachine.LogCLI(err.Error(), 1)
			return
		}
		//validate basedOn is itself valid (merged)
		validBase := false
		//add an exception for first patch which has no base
		if unmarshalled.BasedOn ==
			"5118a21b982bc5611e0aaad96330da21d0fbe0913c1a5b389d6e174f76331f11" {
			//todo: verify that this is the first patch in this repo and signed by a maintainer
			validBase = true
		}
		if !validBase {
			if base, ok := repo.getPatch(unmarshalled.BasedOn); ok {
				maintainer := identity.IsMaintainer(base.Maintainer)
				validBase = maintainer && base.Maintainer != base.CreatedBy
				if !validBase {
					mindmachine.LogCLI("Invalid Base Patch", 4)
					return h, false
				}
			}
		}
		if validBase { //todo OPTIONAL if we get spam: && identity.IsUSH(patch.CreatedBy)
			hs, err := repo.upsert(Patch{
				RepoName:  unmarshalled.RepoName,
				CreatedBy: event.PubKey,
				Diff:      bs,
				UID:       unmarshalled.UID,
				Problem:   unmarshalled.Problem,
				BasedOn:   unmarshalled.BasedOn,
				Sequence:  1,
			})
			if err != nil {
				mindmachine.LogCLI(err, 1)
				return
			}
			return hs, true
		}
	}
	return
}

func rebuildFromShards(k *Kind641002) (Kind641002, bool) {
	events, ok := nostrelay.FetchEventPack(k.Shards)
	if ok {
		var diff []byte
		for _, event := range events {
			var unmarshalled Kind641003
			err := json.Unmarshal([]byte(event.Content), &unmarshalled)
			if err != nil {
				fmt.Println(event)
				mindmachine.LogCLI("could not unmarshal event", 1)
				return Kind641002{}, false
			}
			diff = append(diff, unmarshalled.Diff...)
		}
		bs, _ := hex.DecodeString(fmt.Sprintf("%s", diff))
		computed := mindmachine.Sha256(bs)
		if computed != k.UID {
			mindmachine.LogCLI("rebuilt diff has a different hash to the transmitted UID, probably corrupt like our leaders", 1)
			return Kind641002{}, false
		}
		r := *k
		r.Diff = fmt.Sprintf("%x", bs)
		return r, true
	}
	return Kind641002{}, false
}

func handleNewRepo(e mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	if identity.IsMaintainer(e.PubKey) {
		var unmarshalled Kind641000
		err := json.Unmarshal([]byte(e.Content), &unmarshalled)
		if err != nil {
			return
		}
		if len(unmarshalled.RepoName) > 4 && len(unmarshalled.Problem) == 64 {
			if _, exists := currentState.data[unmarshalled.RepoName]; !exists {
				currentState.data[unmarshalled.RepoName] = makeRepo(unmarshalled.RepoName)
				ignitionPatch := Patch{
					RepoName:   unmarshalled.RepoName,
					CreatedBy:  e.PubKey,
					Maintainer: e.PubKey,
					Diff: []uint8{0x64, 0x69, 0x66, 0x66, 0x20, 0x2d, 0x2d, 0x67, 0x69, 0x74, 0x20, 0x61, 0x2f,
						0x69, 0x67, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x20, 0x62, 0x2f, 0x69, 0x67, 0x6e, 0x69, 0x74,
						0x69, 0x6f, 0x6e, 0xa, 0x6e, 0x65, 0x77, 0x20, 0x66, 0x69, 0x6c, 0x65, 0x20, 0x6d, 0x6f, 0x64, 0x65,
						0x20, 0x31, 0x30, 0x30, 0x36, 0x34, 0x34, 0xa, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x20, 0x30, 0x30, 0x30,
						0x30, 0x30, 0x30, 0x30, 0x2e, 0x2e, 0x30, 0x33, 0x66, 0x62, 0x64, 0x39, 0x62, 0xa, 0x2d, 0x2d, 0x2d,
						0x20, 0x2f, 0x64, 0x65, 0x76, 0x2f, 0x6e, 0x75, 0x6c, 0x6c, 0xa, 0x2b, 0x2b, 0x2b, 0x20, 0x62, 0x2f,
						0x69, 0x67, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0xa, 0x40, 0x40, 0x20, 0x2d, 0x30, 0x2c, 0x30, 0x20,
						0x2b, 0x31, 0x20, 0x40, 0x40, 0xa, 0x2b, 0x54, 0x68, 0x69, 0x73, 0x20, 0x66, 0x69, 0x6c, 0x65, 0x20,
						0x73, 0x68, 0x6f, 0x75, 0x6c, 0x64, 0x20, 0x62, 0x65, 0x20, 0x72, 0x65, 0x6d, 0x6f, 0x76, 0x65, 0x64,
						0x20, 0x77, 0x69, 0x74, 0x68, 0x20, 0x74, 0x68, 0x65, 0x20, 0x66, 0x69, 0x72, 0x73, 0x74, 0x20, 0x70,
						0x61, 0x74, 0x63, 0x68, 0x2e, 0xa},
					UID:       "5118a21b982bc5611e0aaad96330da21d0fbe0913c1a5b389d6e174f76331f11",
					BasedOn:   "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b",
					Height:    0,
					CreatedAt: mindmachine.CurrentState().Processing.Height,
					Conflicts: false,
					Sequence:  1,
				}
				if hs, err := currentState.data[unmarshalled.RepoName].upsert(ignitionPatch); err != nil {
					mindmachine.LogCLI(err.Error(), 1)
					return
				} else {
					return hs, true
				}
			}
		}
	}
	return
}
