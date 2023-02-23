package identity

import (
	"encoding/json"
	"os"
	"sort"
	"sync"

	"github.com/sasha-s/go-deadlock"

	"mindmachine/database"
	"mindmachine/mindmachine"
)

type db struct {
	data  map[mindmachine.Account]Identity
	mutex *deadlock.Mutex
}

var currentState = db{
	data:  make(map[mindmachine.Account]Identity),
	mutex: &deadlock.Mutex{},
}

// StartDb starts the database for this mind (the Mind-state). It blocks until the database is ready to use.
func StartDb(terminate chan struct{}, wg *sync.WaitGroup) {
	if !mindmachine.RegisterMind([]int64{0, 640400, 640402, 640404, 640406, 640408}, "identity", "identity") {
		mindmachine.LogCLI("Could not register Identity Mind", 0)
	}
	// we need a channel to listen for a successful database start
	ready := make(chan struct{})
	// now we can start the database in a new goroutine
	go start(terminate, wg, ready)
	// when the database has started, the goroutine will close the `ready` channel.
	<-ready //This channel listener blocks until closed by `startDb`.
	mindmachine.LogCLI("Identity Mind has started", 4)
}

func start(terminate chan struct{}, wg *sync.WaitGroup, ready chan struct{}) {
	// We add a delta to the provided waitgroup so that upstream knows when the database has been safely shut down
	wg.Add(1)
	// Load current shares from disk
	c, ok := database.Open("identity", "current")
	if ok {
		currentState.restoreFromDisk(c)
	}
	insertIgnitionState()
	close(ready)
	<-terminate
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	b, err := json.MarshalIndent(currentState.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	database.Write("identity", "current", b)
	currentState.takeSnapshot()
	wg.Done()
	mindmachine.LogCLI("Identity Mind has shut down", 4)
}

func insertIgnitionState() {
	//todo preflight
	ignitionAccount := getLatestIdentity(mindmachine.IgnitionAccount)
	if len(ignitionAccount.UniqueSovereignBy) == 0 {
		ignitionAccount.UniqueSovereignBy = "1Humanityrvhus5mFWRRzuJjtAbjk2qwww"
		ignitionAccount.MaintainerBy = "1Humanityrvhus5mFWRRzuJjtAbjk2qwww"
		currentState.upsert(mindmachine.IgnitionAccount, ignitionAccount)
	}
}
func getLatestIdentity(account mindmachine.Account) Identity {
	id, ok := currentState.data[account]
	if !ok {
		return Identity{}
	}
	return id
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

// takeSnapshot calculates a hash (and gets the total sequence) at the current state. It also stores the state in the
//database, indexed by hash of the state. It returns the hash and sequence.
func (s *db) takeSnapshot() mindmachine.HashSeq {
	hs := hashSeq(s.data)
	b, err := json.MarshalIndent(s.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	//write the identity state to file under the current hash
	database.Write("identity", hs.Hash, b)
	database.Write("identity", "current", b)
	return hs
}

func hashSeq(m map[mindmachine.Account]Identity) (hs mindmachine.HashSeq) {
	hs.Mind = "identity"
	var accounts []mindmachine.Account
	for account, _ := range m {
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i] > accounts[j]
	})
	var toHash []any
	for _, account := range accounts {
		ident, _ := m[account]
		hs.Sequence = hs.Sequence + ident.Sequence
		toHash = append(toHash,
			ident.Account,
			ident.Name,
			ident.About,
			ident.UniqueSovereignBy,
			ident.MaintainerBy,
			ident.Sequence,
			ident.Picture)
		for _, strings := range ident.OpReturnAddr {
			for _, s := range strings {
				toHash = append(toHash, s)
			}
		}
		for _, pubkey := range ident.Pubkeys {
			toHash = append(toHash, pubkey)
		}
		for _, legacy := range ident.LegacyIdentities {
			toHash = append(toHash,
				legacy.Username,
				legacy.Platform,
				legacy.EvidenceURL)
		}
		for s, _ := range ident.CharacterVouchedForBy {
			toHash = append(toHash, s)
		}
	}
	for _, d := range toHash {
		if err := hs.AppendData(d); err != nil {
			mindmachine.LogCLI(err, 0)
		}
	}
	hs.S256()
	hs.CreatedAt = mindmachine.CurrentState().Processing.Height
	return
}

func GetMap() map[mindmachine.Account]Identity {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	m := make(map[mindmachine.Account]Identity)
	for account, id := range currentState.data {
		m[account] = id
	}
	return m
}

func IsMaintainer(account mindmachine.Account) bool {
	id := getLatestIdentity(account)
	if len(id.MaintainerBy) > 0 {
		return true
	}
	return false
}

func IsUSH(account mindmachine.Account) bool {
	id := getLatestIdentity(account)
	if len(id.UniqueSovereignBy) > 0 {
		return true
	}
	return false
}

func (s *db) upsert(account mindmachine.Account, identity Identity) {
	identity.Account = account
	s.data[account] = identity
}
