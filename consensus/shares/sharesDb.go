package shares

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/sasha-s/go-deadlock"

	"mindmachine/database"
	"mindmachine/mindmachine"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type db struct {
	data  map[mindmachine.Account]Share
	mutex *deadlock.Mutex
}

var currentState = db{
	data:  make(map[mindmachine.Account]Share),
	mutex: &deadlock.Mutex{},
}

func StartDb(terminate chan struct{}, wg *sync.WaitGroup) {
	if !mindmachine.RegisterMind([]int64{640200, 640202, 640204, 640206}, "shares", "shares") {
		mindmachine.LogCLI("Could not register Shares Mind", 0)
	}
	// we need a channel to listen for a successful database start
	ready := make(chan struct{})
	// now we can start the database in a new goroutine
	go start(terminate, wg, ready)
	// when the database has started, the goroutine will close the `ready` channel.
	<-ready //This channel listener blocks until closed by `startDb`.
	mindmachine.LogCLI("Shares Mind has started", 4)
}

func start(terminate chan struct{}, wg *sync.WaitGroup, ready chan struct{}) {
	wg.Add(1)
	// Load current shares from disk
	c, ok := database.Open("shares", "current")
	if ok {
		currentState.restoreFromDisk(c)
	}
	close(ready)
	<-terminate
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	b, err := json.MarshalIndent(currentState.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	database.Write("shares", "current", b)
	currentState.takeSnapshot()
	wg.Done()
	mindmachine.LogCLI("Shares Mind has shut down", 4)
}

func (s *db) restoreFromDisk(f *os.File) {
	s.mutex.Lock()
	err := json.NewDecoder(f).Decode(&s.data)
	if err != nil {
		if err.Error() != "EOF" {
			mindmachine.LogCLI(err.Error(), 0)
		}
	}
	s.mutex.Unlock()
	err = f.Close()
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
}

func getHistoricalStateFromDisk(hash mindmachine.S256Hash) (*db, bool) {
	smap := &db{}
	f, ok := database.Open("shares", hash)
	if !ok {
		return smap, false
	}
	defer f.Close()
	smap.mutex = &deadlock.Mutex{}
	smap.restoreFromDisk(f)
	return smap, true
}

// takeSnapshot calculates a hash (and gets the total sequence) at the current state. It also stores the state in the
//database, indexed by hash of the state. It returns the hash and sequence.
func (s *db) takeSnapshot() mindmachine.HashSeq {
	hs := hashSeq(s.data)
	b, err := json.MarshalIndent(s.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	//write the share state to file
	database.Write("shares", hs.Hash, b)
	database.Write("shares", "current", b)
	var seq int64
	for _, share := range s.data {
		seq = seq + share.Sequence
	}
	return hs
}

func hashSeq(m map[mindmachine.Account]Share) (hs mindmachine.HashSeq) {
	hs.Mind = "shares"
	var sortedAccounts []mindmachine.Account
	for account := range m {
		sortedAccounts = append(sortedAccounts, account)
	}
	sort.Slice(sortedAccounts, func(i, j int) bool {
		return sortedAccounts[i] > sortedAccounts[j]
	})
	var toHash []any
	for _, account := range sortedAccounts {
		if share, ok := m[account]; !ok {
			mindmachine.LogCLI("this should not happen", 0)
		} else {
			hs.Sequence = hs.Sequence + share.Sequence
			toHash = append(toHash,
				account,
				share.Sequence,
				share.LastLtChange,
				share.LeadTimeLockedShares,
				share.LeadTime,
				share.LeadTimeUnlockedShares)
			for _, expens := range share.Expenses {
				toHash = append(toHash,
					expens.UID,
					expens.Problem,
					expens.WitnessedAt,
					expens.Amount,
					expens.Approved,
					expens.Solution,
					expens.BlackballPermille,
					expens.RatifyPermille)
				for rv, _ := range expens.Ratifiers {
					toHash = append(toHash, rv)
				}
				for bbv, _ := range expens.Blackballers {
					toHash = append(toHash, bbv)
				}
			}
		}
	}
	for _, d := range toHash {
		err := hs.AppendData(d)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 0)
		}
	}
	hs.S256()
	hs.CreatedAt = mindmachine.CurrentState().Processing.Height
	return
}

// getMapHash takes the provided map of account data and returns a deterministic hash
// this does not vary even if the names of the Types used do vary through different
// iterations of the codebase. This MUST return the same hash even if Type definitions change.
func getMapHash(m map[mindmachine.Account]Share) mindmachine.S256Hash {
	var sortedAccounts []mindmachine.Account
	for account := range m {
		sortedAccounts = append(sortedAccounts, account)
	}
	sort.Slice(sortedAccounts, func(i, j int) bool {
		return sortedAccounts[i] > sortedAccounts[j]
	})
	buf := &bytes.Buffer{}
	for _, account := range sortedAccounts {
		buf.WriteString(fmt.Sprint(account, m[account].LeadTimeLockedShares, m[account].LeadTime, m[account].LastLtChange, m[account].Sequence))
		for _, expens := range m[account].Expenses {
			buf.WriteString(expens.hash())
		}
	}
	return mindmachine.Sha256(buf.Bytes())
}

func (s *db) upsert(account mindmachine.Account, share Share) {
	s.data[account] = share
}

func StateForAccount(account mindmachine.Account) Share {
	s, ok := currentState.data[account]
	if !ok {
		s = Share{}
	}
	return s
}

func HistoricalStateForAccount(account mindmachine.Account, stateHash mindmachine.S256Hash) (Share, error) {
	smap, ok := getHistoricalStateFromDisk(stateHash)
	if ok {
		share, ok := smap.data[account]
		if ok {
			return share, nil
		}
	}
	return Share{}, fmt.Errorf("we do not have a share state for the provided hash")
}

func totalVotePower() mindmachine.VotePower {
	m := currentState.data
	var vp mindmachine.VotePower
	for _, share := range m {
		vp = vp + share.AbsoluteVotePower()
	}
	if vp > (9223372036854775807 / 5) {
		mindmachine.LogCLI("We are 20% of the way to an overflow bug, better do something", 0)
	}
	return vp
}

func TotalVotepowerAtState(stateHash mindmachine.S256Hash) (mindmachine.VotePower, error) {
	shares, ok := getHistoricalStateFromDisk(stateHash)
	if ok {
		var totalVP mindmachine.VotePower
		for _, share := range shares.data {
			totalVP = totalVP + share.AbsoluteVotePower()
		}
		return totalVP, nil
	}
	return 0, fmt.Errorf("we do not have a share state for the provided hash")
}

func MapOfCurrentState() map[mindmachine.Account]Share {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	m := make(map[mindmachine.Account]Share)
	for account, share := range currentState.data {
		m[account] = share
	}
	return m
}

func AccountsWithVotepower() map[mindmachine.Account]int64 {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	return accountsWithVotepower()
}

func accountsWithVotepower() (a map[mindmachine.Account]int64) {
	a = make(map[mindmachine.Account]int64)
	for account, share := range currentState.data {
		if share.AbsoluteVotePower() > 0 {
			a[account] = share.AbsoluteVotePower()
		}
	}
	return
}

func current(account mindmachine.Account) Share {
	if shares, ok := currentState.data[account]; ok {
		return shares
	}
	return Share{}
}

func HashOfCurrentState() mindmachine.S256Hash {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	return hashSeq(currentState.data).Hash
}

func DoWeHaveStateForThisHash(s mindmachine.S256Hash) bool {
	_, ok := getHistoricalStateFromDisk(s)
	return ok
}

func MapOfStateAtHash(hash string) (r map[mindmachine.Account]Share, ok bool) {
	r = make(map[mindmachine.Account]Share)
	m, ok := getHistoricalStateFromDisk(hash)
	if ok {
		for account, share := range m.data {
			r[account] = share
		}
	}
	return
}

func VotePowerForAccount(account mindmachine.Account) mindmachine.VotePower {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	vp := votePowerForAccount(account)
	return vp
}

func votePowerForAccount(account mindmachine.Account) mindmachine.VotePower {
	s := current(account)
	return s.AbsoluteVotePower()
}

// Permille returns the total Permille for the provided accounts
func Permille(accounts map[mindmachine.Account]struct{}) mindmachine.VotePower {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	var accountTotal int64
	for account := range accounts {
		accountTotal = accountTotal + votePowerForAccount(account)
	}
	total := totalVotePower()
	return mindmachine.Permille(accountTotal, total)
}
