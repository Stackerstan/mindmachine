package sequence

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
	data  map[mindmachine.Account]Sequence
	mutex *deadlock.Mutex
}

var currentState = db{
	data:  make(map[mindmachine.Account]Sequence),
	mutex: &deadlock.Mutex{},
}

// StartDb starts the database for this mind (the Mind-state). It blocks until the database is ready to use.
func StartDb(terminate chan struct{}, wg *sync.WaitGroup) {

	// we need a channel to listen for a successful database start
	ready := make(chan struct{})
	// now we can start the database in a new goroutine
	go start(terminate, wg, ready)
	// when the database has started, the goroutine will close the `ready` channel.
	<-ready //This channel listener blocks until closed by `startDb`.
	mindmachine.LogCLI("Sequence Mind has started", 4)
}

func start(terminate chan struct{}, wg *sync.WaitGroup, ready chan struct{}) {
	// We add a delta to the provided waitgroup so that upstream knows when the database has been safely shut down
	wg.Add(1)
	// Load current shares from disk
	c, ok := database.Open("sequence", "current")
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
	database.Write("sequence", "current", b)
	currentState.takeSnapshot()
	wg.Done()
	mindmachine.LogCLI("Sequence Mind has shut down", 4)
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
	database.Write("sequence", hs.Hash, b)
	return hs
}

func hashSeq(m map[mindmachine.Account]Sequence) (hs mindmachine.HashSeq) {
	hs.Mind = "sequence"
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
			ident.Sequence)
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

func AllSequences() (s []Sequence) {
	for _, sequence := range currentState.data {
		s = append(s, sequence)
	}
	return
}
