package mindstate

import (
	"fmt"
	"os"
	"sync"

	"github.com/fiatjaf/go-nostr"
	jsoniter "github.com/json-iterator/go"
	"mindmachine/consensus/shares"

	"github.com/sasha-s/go-deadlock"
	"mindmachine/database"
	"mindmachine/mindmachine"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var dbReady = make(chan struct{})

var currentState = db{
	data:  make(map[mindmachine.S256Hash]VpssData),
	mutex: &deadlock.Mutex{},
}

func StartDb(terminate chan struct{}, wg *sync.WaitGroup) {
	if !mindmachine.RegisterMind([]int64{640000}, "vpss", "vpss") {
		mindmachine.LogCLI("Could not register VPSS Mind", 0)
	}
	ready := make(chan struct{})
	go startDb(terminate, wg, ready)
	<-ready
	mindmachine.LogCLI("MindState Mind (VPSS) has started", 4)
}

func startDb(terminate chan struct{}, wg *sync.WaitGroup, ready chan struct{}) {
	wg.Add(1)
	// load current data from disk
	c, ok := database.Open("vpss", "current")
	if ok {
		currentState.restoreFromDisk(c)
	}
	close(dbReady)
	close(ready)
	<-terminate // we sit here until shutdown is called
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	b, err := json.MarshalIndent(currentState.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	database.Write("vpss", "current", b)
	wg.Done()
	mindmachine.LogCLI("MindState Mind (VPSS) has shut down", 4)
}

func (s *db) restoreFromDisk(f *os.File) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	err := json.NewDecoder(f).Decode(&s.data)
	if err != nil {
		if err.Error() != "EOF" {
			mindmachine.LogCLI(err.Error(), 0)
		}
	}
	err = f.Close()
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
}

func (s *db) writeToDisk() {
	b, err := json.MarshalIndent(s.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	database.Write("vpss", "current", b)
}

func GetFullDB() map[mindmachine.S256Hash]VpssData {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	return currentState.data
}

//GetLatestStates returns the latest >500 permille states with a correct sigchain back to the ignition event and where the Minds actually have the data for this state.
func GetLatestStates() (l map[string]MindState) {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	//for hash, data := range currentState.data {
	//	if data.Mind == "problems" && data.Sigchain && data.HaveMindState && data.Sequence == 12 {
	//		fmt.Printf("\n%s\n%#v\n", hash, data)
	//	}
	//
	//}
	return getLatestStatus(true)
}

//getLatestStatus withState set to true = we don't return any states if the corresponding Mind doesn't actually have the state.
//When we are verifying an eventpack worked to catch us up, we don't want to include vpss where the Mind doesn't actually have the state.
//When seeing if we need to catch up with peers, we just get the latest status whether we have the state or not.
func getLatestStatus(withState bool) (l map[string]MindState) {
	//todo this needs to include the height of our latest >500 permille, not just imply that it's the current height.
	//problem: when we have network splits, we think that we can't roll back to our last >500 permille state because "we have >500 permille at the current state"
	l = make(map[string]MindState)
	filtered := make(map[string]VpssData)
	for _, data := range currentState.data {
		if data.Sigchain {
			if data.Permille > 500 {
				if current, ok := filtered[data.Mind]; ok {
					if data.Sequence > current.Sequence {
						if !withState {
							filtered[data.Mind] = data
						}
						if withState && data.HaveMindState {
							filtered[data.Mind] = data
						}
					}
				} else {
					if !withState {
						filtered[data.Mind] = data
					}
					if withState && data.HaveMindState {
						filtered[data.Mind] = data
					}
				}

			}
		}
	}
	for _, data := range filtered {
		ms := MindState{
			Mind:     data.Mind,
			State:    data.MindStateHash,
			Height:   data.Height,
			Sequence: data.Sequence,
			Permille: data.Permille,
		}
		for _, proof := range data.Proofs {
			//todo when veryfing state don't check order of signing accounts or it could fail
			ms.SigningAccounts = append(ms.SigningAccounts, proof.PubKey)
			l[ms.Mind] = ms
		}
	}
	return
}

func getSignedandAvailableVotePowerAtNailedStateWithLatestGT500(vpd VpssData) (signed, available int64) {
	l := getLatestStatus(true)
	lSharesMind, ok := l["shares"]
	if !ok {
		mindmachine.LogCLI("this should not happen", 0)
	}
	lSharesState := lSharesMind.State
	if len(lSharesState) != 64 {
		mindmachine.LogCLI("this shouldn't happen", 0)
	}
	m, ok := shares.MapOfStateAtHash(lSharesState)
	if !ok {
		mindmachine.LogCLI("this should not happen", 0)
	}
	for _, proof := range vpd.Proofs {
		s, ok := m[proof.PubKey]
		if ok {
			signed += s.AbsoluteVotePower()
		}
	}
	for _, share := range m {
		available += share.AbsoluteVotePower()
	}
	return
}

func getSignedandAvailableVotePowerAtNailedState(vpd VpssData) (signed, available int64) {
	m, ok := shares.MapOfStateAtHash(vpd.NailedTo)
	if !ok {
		mindmachine.LogCLI("this should not happen", 0)
		return
	}
	for _, proof := range vpd.Proofs {
		s, ok := m[proof.PubKey]
		if ok {
			signed += s.AbsoluteVotePower()
		}
	}
	for _, share := range m {
		available += share.AbsoluteVotePower()
	}
	return
}

//sigchain is a sanity check on whether we should trust this state is really immutable or not.
//It returns true if all signers in the proposed state that were present in the most recent 1000 permille state
//have signed the proposed state, and false otherwise. If none of the signers from the last 1000 permille state
//exist, it also returns false.
func sigchain(vpd VpssData) bool {
	if vpd.Sigchain {
		return true
	}
	if vpd.Mind == "shares" && vpd.Sequence == 1 {
		//this is the ignition vpss
		return true
	}
	if vpd.Mind != "shares" {
		shareVpss, ok := currentState.data[vpd.NailedTo]
		if !ok {
			fmt.Printf("\nWant data in current state for %s\n\nCurrent data table:\n%#v\n", vpd.NailedTo+vpd.NailedTo, currentState.data)
			mindmachine.LogCLI("sigchain failed to validate nailed share state", 1)
			return false
		}
		if !sigchain(shareVpss) {
			return false
		}
	}
	//get a list of voters at the most recent 1000 permille state
	l := getLatest1000PermilleState("shares")
	mHistorical, ok := shares.MapOfStateAtHash(l.MindStateHash)
	if !ok {
		fmt.Printf("\n\n--- VPSS data that triggered the query: ---\n%#v\n\n--- VPSS data for the Share state we don't have: ---\n%#v\n", vpd, l)
		mindmachine.LogCLI("We should have a Share state for this hash but we do not. For debug, Halt now and find data in preceding lines.", 1)
		return false
	}
	//get intersection of voters in vpd and 1000 permille state
	//return true if all of them signed it, false if they did not
	mCurrent, ok := shares.MapOfStateAtHash(vpd.NailedTo)
	if !ok {
		return false
	}
	var filtered = make(map[mindmachine.Account]shares.Share)
	for account, share := range mCurrent {
		if share.AbsoluteVotePower() > 0 {
			filtered[account] = share
		}
	}

	var intersection = make(map[mindmachine.Account]struct{})
	for account, _ := range filtered {
		share, ok := mHistorical[account]
		if ok {
			if share.AbsoluteVotePower() > 0 {
				intersection[account] = struct{}{}
			}
		}
	}

	var signers = make(map[mindmachine.Account]struct{})
	for _, proof := range vpd.Proofs {
		signers[proof.PubKey] = struct{}{}
	}
	if len(signers) == 0 {
		return false
	}

	var unsigned int64
	for account := range intersection {
		if _, ok := signers[account]; !ok {
			unsigned++
		}
	}
	if unsigned > 0 {
		return false
	}
	return true
}

func getLatest1000PermilleState(mind string) VpssData {
	var filtered []VpssData
	for _, data := range currentState.data {
		if data.Mind == mind {
			if data.Permille == 1000 {
				filtered = append(filtered, data)
			}
		}
	}
	var latest = VpssData{Height: 0, Sequence: 0}
	for _, data := range filtered {
		if data.Height > latest.Height {
			if data.Sequence > latest.Sequence {
				latest = data
			}
		}
	}
	return latest
}

func withoutDuplicateSigners(proofs []nostr.Event) []nostr.Event {
	m := make(map[string]nostr.Event)
	for _, proof := range proofs {
		m[proof.PubKey] = proof
	}
	var noDuplicates []nostr.Event
	for _, event := range m {
		noDuplicates = append(noDuplicates, event)
	}
	return noDuplicates
}
