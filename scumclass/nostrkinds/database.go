package nostrkinds

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/sasha-s/go-deadlock"
	"mindmachine/database"
	"mindmachine/mindmachine"
)

type db struct {
	data  map[int64]Kind
	mutex *deadlock.Mutex
}

var currentState = db{
	data:  make(map[int64]Kind),
	mutex: &deadlock.Mutex{},
}

var startMutex = &deadlock.Mutex{}
var started = false

// StartDb starts the database for this mind (the Mind-state). It blocks until the database is ready to use.
func StartDb(terminate chan struct{}, wg *sync.WaitGroup) {
	startMutex.Lock()
	defer startMutex.Unlock()
	if !started {
		started = true
		if !mindmachine.RegisterMind([]int64{641800}, "nostrkinds", "nostrkinds") {
			mindmachine.LogCLI("Could not register NostrKinds Mind", 0)
		}
		// we need a channel to listen for a successful database start
		ready := make(chan struct{})
		// now we can start the database in a new goroutine
		go start(terminate, wg, ready)
		// when the database has started, the goroutine will close the `ready` channel.
		<-ready //This channel listener blocks until closed by `start`.
		mindmachine.LogCLI("NostrKinds Mind (scum class) has started", 4)
	}
}

// start opens the database from disk (or creates it). It closes the `ready` channel once the database is ready to
// handle queries, and shuts down safely when the terminate channel is closed. Any upstream functions that need to
// know when the database has been shut down should wait on the provided waitgroup.
func start(terminate chan struct{}, wg *sync.WaitGroup, ready chan struct{}) {
	// We add a delta to the provided waitgroup so that upstream knows when the database has been safely shut down
	wg.Add(1)
	// here we are opening the databases so that they can be used throughout this mind.
	c, ok := database.Open("nostrkinds", "current")
	if ok {
		currentState.restoreFromDisk(c)
	}

	close(ready)
	// The database has been started. Now we wait on the terminate channel
	// until upstream closes it (telling us to shut down).
	<-terminate
	//give the handler time to finish
	time.Sleep(time.Second)
	// We are shutting down, so we need to safely close the databases.
	writeDb()
	//currentState.mutex.Lock()
	//defer currentState.mutex.Unlock()
	//b, err := json.MarshalIndent(currentState.data, "", " ")
	//if err != nil {
	//	mindmachine.LogCLI(err.Error(), 0)
	//}
	//database.Write("eventbucket", "current", b)
	//Tell upstream that we have finished shutting down the databases
	wg.Done()
	mindmachine.LogCLI("NostrKinds Mind (scum class) has shut down", 4)
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

func (s *db) upsert(i Kind) {
	s.data[i.Kind] = i
}

func GetAll() map[int64]Kind {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	return currentState.data
}

func writeDb() {
	b, err := json.MarshalIndent(currentState.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 1)
	} else {
		database.Write("nostrkinds", "current", b)
	}
}
