package mindstate

import (
	"fmt"

	"mindmachine/consensus/shares"
	"mindmachine/mindmachine"
)

//todo we probably need to revoke VpssData.HaveMindState if we roll back a state

//RegisterState tells the vpss handler that we have the corresponding Mind-state for this Mind-hash
func RegisterState(v mindmachine.Event) bool {
	<-dbReady
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	var unmarshalled mindmachine.Kind640000
	err := json.Unmarshal([]byte(v.Content), &unmarshalled)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 2)
		return false
	}
	existing, ok := currentState.data[unmarshalled.Hash+unmarshalled.ShareHash]
	if !ok {
		existing = VpssData{
			Mind:          unmarshalled.Mind,
			MindStateHash: unmarshalled.Hash,
			Sequence:      unmarshalled.Sequence,
			Height:        unmarshalled.Height,
			NailedTo:      unmarshalled.ShareHash,
		}
		if existing.Mind == "shares" {
			existing.NailedTo = existing.MindStateHash
		}
	}
	if v.PubKey == mindmachine.MyWallet().Account {
		existing.HaveMindState = true
		existing.HaveNailedState = true
		currentState.data[unmarshalled.Hash+unmarshalled.ShareHash] = existing
		return true
	}
	return false
}

//HandleVPSS returns false if the VPSS fails, and true if it succeeds
func HandleVPSS(v mindmachine.Event) (err error, b bool) {
	<-dbReady
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	ok, _ := v.CheckSignature()
	if !ok {
		mindmachine.LogCLI("this should not happen", 0)
		return
	}
	var unmarshalled mindmachine.Kind640000
	err = json.Unmarshal([]byte(v.Content), &unmarshalled)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 2)
		return
	}
	//If we are not the signer, and the signer does not have any votepower, ignore it.
	signerVotePower := shares.VotePowerForAccount(v.PubKey)
	if signerVotePower < 1 {
		fmt.Println(68)
		return
	}

	//Aggregate all the VPSSs for this state and add up the permille
	existing, ok := currentState.data[unmarshalled.Hash+unmarshalled.ShareHash]
	if !ok {
		existing = VpssData{
			Mind:          unmarshalled.Mind,
			MindStateHash: unmarshalled.Hash,
			Sequence:      unmarshalled.Sequence,
			Height:        unmarshalled.Height,
			NailedTo:      unmarshalled.ShareHash,
		}
		if existing.Mind == "shares" {
			existing.NailedTo = existing.MindStateHash
		}
	}
	var thisAccountHasAlreadySigned bool = false
	//if v.PubKey == mindmachine.MyWallet().Account {
	//existing.HaveMindState = true
	//existing.HaveNailedState = true
	for _, proof := range existing.Proofs {
		if proof.PubKey == v.PubKey {
			thisAccountHasAlreadySigned = true
		}
	}
	//}
	//todo preflight
	if existing.Sequence == 1 && existing.Mind == "shares" && v.PubKey == mindmachine.IgnitionAccount {
		//this is the ignition VPSS
		existing.HaveMindState = true
		existing.HaveNailedState = true
	}
	if !existing.HaveNailedState && existing.Mind != "shares" {
		if !shares.DoWeHaveStateForThisHash(existing.NailedTo) {
			return
		}
		existing.HaveNailedState = true
	}

	if signerVotePower > 0 {
		//todo check if signer has votepower at the nailedState, not at the current state?
		existing.Proofs = append(existing.Proofs, v.Nostr())
		existing.Proofs = withoutDuplicateSigners(existing.Proofs)
	}

	//Even if we are greater than 500 permille, we should see if all the voters at our last 1000permille state
	//who are still around in this state have signed it, if not then we should not consider it immutable yet.
	existing.Sigchain = sigchain(existing)

	if existing.HaveNailedState {
		existing.VpSigned, existing.VpAvailable = getSignedandAvailableVotePowerAtNailedState(existing)
	}
	//handle shares mind vpss when we don't have the mind-state for this hash
	if !existing.HaveNailedState {
		if existing.Mind != "shares" {
			mindmachine.LogCLI("this should not happen", 0)
		}
		existing.VpSigned, existing.VpAvailable = getSignedandAvailableVotePowerAtNailedStateWithLatestGT500(existing)
	}
	existing.Permille = mindmachine.Permille(existing.VpSigned, existing.VpAvailable)

	//if existing.Permille > 0 {
	currentState.data[unmarshalled.Hash+unmarshalled.ShareHash] = existing
	currentState.writeToDisk()
	if thisAccountHasAlreadySigned {
		//fmt.Println(134)
		return nil, false
	}
	if existing.HaveMindState && existing.HaveNailedState && !thisAccountHasAlreadySigned {
		return nil, true
	} else {
		return nil, false
	}
}
