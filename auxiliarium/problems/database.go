/*
Package problems is a General Service Mind which retains a Mind-state containing all Problems logged by Participants.
*/

package problems

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/sasha-s/go-deadlock"
	"mindmachine/database"
	"mindmachine/messaging/nostrelay"
	"mindmachine/mindmachine"
)

type db struct {
	data  map[mindmachine.S256Hash]Problem
	mutex *deadlock.Mutex
}

var currentState = db{
	data:  make(map[mindmachine.S256Hash]Problem),
	mutex: &deadlock.Mutex{},
}

func (d *db) current() (m map[mindmachine.S256Hash]Problem) {
	m = make(map[mindmachine.S256Hash]Problem)
	d.mutex.Lock()
	defer d.mutex.Unlock()
	for hash, item := range currentState.data {
		m[hash] = item
	}
	return
}

// StartDb starts the database for this mind (the Mind-state). It blocks until the database is ready to use.
func StartDb(terminate chan struct{}, wg *sync.WaitGroup) {
	//ignition(true)
	if !mindmachine.RegisterMind([]int64{640800, 640802}, "problems", "problems") {
		mindmachine.LogCLI("Could not register Problems Mind", 0)
	}
	// we need a channel to listen for a successful database start
	ready := make(chan struct{})
	// now we can start the database in a new goroutine
	go start(terminate, wg, ready)
	// when the database has started, the goroutine will close the `ready` channel.
	<-ready //This channel listener blocks until closed by `start`.
	mindmachine.LogCLI("Problem Tracker Mind has started", 4)
}

// start opens the database from disk (or creates it). It closes the `ready` channel once the database is ready to
// handle queries, and shuts down safely when the terminate channel is closed. Any upstream functions that need to
// know when the database has been shut down should wait on the provided waitgroup.
func start(terminate chan struct{}, wg *sync.WaitGroup, ready chan struct{}) {
	// We add a delta to the provided waitgroup so that upstream knows when the database has been safely shut down
	wg.Add(1)
	// here we are opening the databases so that they can be used throughout this mind.
	c, ok := database.Open("problems", "current")
	if ok {
		currentState.restoreFromDisk(c)
	}
	close(ready)
	if len(currentState.data) == 0 {
		ig := ignition(false)
		nostrelay.PublishEvent(ig[0])
		nostrelay.PublishEvent(ig[1])
		//fmt.Printf("\n%#v\n", mindmachine.ConvertToInternalEvent(&ig[2]))
		fmt.Println(HandleEvent(mindmachine.ConvertToInternalEvent(&ig[2])))
	}
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
	database.Write("problems", "current", b)
	//Tell upstream that we have finished shutting down the databases
	wg.Done()
	mindmachine.LogCLI("Problem Tracker Mind has shut down", 4)
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

func (s *db) upsert(i Problem) {
	s.data[i.UID] = i
}
