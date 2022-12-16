package doki

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
	data  map[mindmachine.S256Hash]Document
	mutex *deadlock.Mutex
}

var currentState = db{
	data:  make(map[mindmachine.S256Hash]Document),
	mutex: &deadlock.Mutex{},
}

// StartDb starts the database for this mind (the Mind-state). It blocks until the database is ready to use.
func StartDb(terminate chan struct{}, wg *sync.WaitGroup) {
	//ignition(true)
	if !mindmachine.RegisterMind([]int64{641200, 641202, 641204}, "doki", "doki") {
		mindmachine.LogCLI("Could not register Doki Mind", 0)
	}
	// we need a channel to listen for a successful database start
	ready := make(chan struct{})
	// now we can start the database in a new goroutine
	go start(terminate, wg, ready)
	// when the database has started, the goroutine will close the `ready` channel.
	<-ready //This channel listener blocks until closed by `start`.
	mindmachine.LogCLI("Doki Mind has started", 4)
}

// start opens the database from disk (or creates it). It closes the `ready` channel once the database is ready to
// handle queries, and shuts down safely when the terminate channel is closed. Any upstream functions that need to
// know when the database has been shut down should wait on the provided waitgroup.
func start(terminate chan struct{}, wg *sync.WaitGroup, ready chan struct{}) {
	// We add a delta to the provided waitgroup so that upstream knows when the database has been safely shut down
	wg.Add(1)
	// here we are opening the databases so that they can be used throughout this mind.
	c, ok := database.Open("doki", "current")
	if ok {
		currentState.restoreFromDisk(c)
	}
	close(ready)
	// The database has been started. Now we wait on the terminate channel
	// until upstream closes it (telling us to shut down).
	<-terminate
	// We are shutting down, so we need to safely close the databases.
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	b, err := json.MarshalIndent(currentState.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	database.Write("doki", "current", b)
	//Tell upstream that we have finished shutting down the databases
	wg.Done()
	mindmachine.LogCLI("Doki Mind has shut down", 4)
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

// takeSnapshot calculates a hash (and gets the total sequence) at the current state. It also stores the state in the
//database, indexed by hash of the state. It returns the hash and sequence.
func (s *db) takeSnapshot() mindmachine.HashSeq {
	hs := hashSeq(s.data)
	b, err := json.MarshalIndent(s.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	//write the database state to file
	database.Write("doki", "current", b)
	var seq int64
	for _, share := range s.data {
		seq = seq + share.Sequence
	}
	return hs
}

func hashSeq(m map[mindmachine.S256Hash]Document) (hs mindmachine.HashSeq) {
	hs.Mind = "doki"
	var sorted []mindmachine.S256Hash
	for account := range m {
		sorted = append(sorted, account)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] > sorted[j]
	})
	var toHash []any
	for _, hash := range sorted {
		if document, ok := m[hash]; !ok {
			mindmachine.LogCLI("this should not happen", 0)
		} else {
			hs.Sequence = hs.Sequence + document.Sequence
			toHash = append(toHash,
				hash,
				document.Sequence,
				document.CreatedBy,
				document.MergedBy)
			for _, patch := range document.Patches {
				toHash = append(toHash, patch.CreatedBy, patch.MergedBy, patch.RejectedBy, patch.EventID, patch.Patch)
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
