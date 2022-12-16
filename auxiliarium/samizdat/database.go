package samizdat

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/sasha-s/go-deadlock"
	"mindmachine/database"
	"mindmachine/mindmachine"
)

//when handling events, organise them into their correct place in the tree

type db struct {
	data  map[string]Samizdat
	mutex *deadlock.Mutex
}

var currentState = db{
	data:  make(map[string]Samizdat),
	mutex: &deadlock.Mutex{},
}

// StartDb starts the database for this mind (the Mind-state). It blocks until the database is ready to use.
func StartDb(terminate chan struct{}, wg *sync.WaitGroup) {
	//ignition(true)
	if !mindmachine.RegisterMind([]int64{1, 5}, "samizdat", "samizdat") {
		mindmachine.LogCLI("Could not register Samizdat Mind", 0)
	}
	// we need a channel to listen for a successful database start
	ready := make(chan struct{})
	// now we can start the database in a new goroutine
	go start(terminate, wg, ready)
	// when the database has started, the goroutine will close the `ready` channel.
	<-ready //This channel listener blocks until closed by `start`.
	mindmachine.LogCLI("Samizdat Mind has started", 4)
}

// start opens the database from disk (or creates it). It closes the `ready` channel once the database is ready to
// handle queries, and shuts down safely when the terminate channel is closed. Any upstream functions that need to
// know when the database has been shut down should wait on the provided waitgroup.
func start(terminate chan struct{}, wg *sync.WaitGroup, ready chan struct{}) {
	// We add a delta to the provided waitgroup so that upstream knows when the database has been safely shut down
	wg.Add(1)
	// here we are opening the databases so that they can be used throughout this mind.
	c, ok := database.Open("samizdat", "current")
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
	database.Write("samizdat", "current", b)
	//Tell upstream that we have finished shutting down the databases
	wg.Done()
	mindmachine.LogCLI("Samizdat Mind has shut down", 4)
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

// takeSnapshot calculates a hash (and gets the total sequence) at the current state.
func (s *db) takeSnapshot() mindmachine.HashSeq {
	hs := s.hashSeq()
	b, err := json.MarshalIndent(s.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	//write the database state to file
	database.Write("samizdat", "current", b)
	return hs
}

func (d *db) hashSeq() (hs mindmachine.HashSeq) {
	stringSlice := d.listify(d.findRoot())
	var byteSlice [][]byte
	for _, id := range stringSlice {
		bs, err := hex.DecodeString(id)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 0)
		}
		byteSlice = append(byteSlice, bs)
	}
	mklRoot := mindmachine.Merkle(byteSlice)[0]
	hs.Mind = "samizdat"
	hs.Sequence = int64(len(byteSlice))
	hs.Hash = fmt.Sprintf("%x", mklRoot)
	hs.CreatedAt = mindmachine.CurrentState().Processing.Height
	return
}

func (m *db) listify(root string) []string {
	var allIDsAsStrings []string
	allIDsAsStrings = append(allIDsAsStrings, root)
	for _, child := range m.data[root].Children {
		allIDsAsStrings = append(allIDsAsStrings, m.listify(child)...)
	}
	return allIDsAsStrings
}

func (m *db) findRoot() (r string) {
	r = "0"
	for s, samizdat := range m.data {
		if samizdat.Parent == "" {
			r = s
		}
	}
	return
}

type Samizdat struct {
	ID       string
	Parent   string
	Children []string
}

func PrintEmAll() {
	for _, samizdat := range currentState.data {
		fmt.Printf("\n%#b\n", samizdat)
	}
}
