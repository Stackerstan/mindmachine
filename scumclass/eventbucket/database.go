package eventbucket

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/sasha-s/go-deadlock"
	"github.com/stackerstan/go-nostr"
	"mindmachine/database"
	"mindmachine/mindmachine"
)

type db struct {
	data  map[mindmachine.S256Hash]Event
	mutex *deadlock.Mutex
}

var currentState = db{
	data:  make(map[mindmachine.S256Hash]Event),
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
		if !mindmachine.RegisterMind([]int64{}, "eventbucket", "eventbucket") {
			mindmachine.LogCLI("Could not register Eventbucket Mind", 0)
		}
		// we need a channel to listen for a successful database start
		ready := make(chan struct{})
		// now we can start the database in a new goroutine
		go start(terminate, wg, ready)
		// when the database has started, the goroutine will close the `ready` channel.
		<-ready //This channel listener blocks until closed by `start`.
		mindmachine.LogCLI("Mindmachine Eventbucket Mind (scum class) has started", 4)
	}
}

// start opens the database from disk (or creates it). It closes the `ready` channel once the database is ready to
// handle queries, and shuts down safely when the terminate channel is closed. Any upstream functions that need to
// know when the database has been shut down should wait on the provided waitgroup.
func start(terminate chan struct{}, wg *sync.WaitGroup, ready chan struct{}) {
	// We add a delta to the provided waitgroup so that upstream knows when the database has been safely shut down
	wg.Add(1)
	// here we are opening the databases so that they can be used throughout this mind.
	c, ok := database.Open("eventbucket", "current")
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
	mindmachine.LogCLI("Mindmachine Eventbucket Mind has shut down", 4)
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

func (s *db) upsert(i Event) {
	s.data[i.EventID] = i
}

func GetAll() map[mindmachine.S256Hash]Event {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	return currentState.data
}

func Count() int64 {
	return int64(len(currentState.data))
}

func GetNumberOfKinds() (k []Kind) {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	kinds := make(map[int64]int64)
	for _, target := range currentState.data {
		kinds[target.Kind]++
	}
	for i, i2 := range kinds {
		k = append(k, Kind{
			Kind:  i,
			Count: i2,
		})
	}
	sort.Slice(k, func(i, j int) bool {
		return k[i].Kind > k[j].Kind
	})
	return
}

var currentOrder []Event
var currentOrderMutex = &deadlock.Mutex{}
var lastCalculated time.Time

func Fetch(id string) (nostr.Event, bool) {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	if e, ok := currentState.data[id]; ok {
		return e.Event, true
	}
	return nostr.Event{}, false
}

func CurrentOrder() []Event {
	if time.Since(lastCalculated) > (time.Minute*2) || len(currentOrder) < 20 {
		calculateMentions()
	}
	currentOrderMutex.Lock()
	defer currentOrderMutex.Unlock()
	return currentOrder
}

func calculateMentions() {
	defer timeTrack(time.Now(), "calculateMentions")
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	currentState.data = resetMentions(currentState.data)
	for _, target := range currentState.data {
		tags := target.Event.Tags.GetAll([]string{"e"})
		for _, tag := range tags {
			if _, ok := currentState.data[tag.Value()]; ok {
				current := currentState.data[tag.Value()]
				current.Mentions++
				current.MentionMap[target.EventID] = struct{}{}
				currentState.data[tag.Value()] = current
			} else {
				currentState.data[tag.Value()] = Event{
					EventID:    tag.Value(),
					Score:      400,
					Mentions:   1,
					MentionMap: make(map[string]struct{}),
				}
				currentState.data[tag.Value()].MentionMap[target.EventID] = struct{}{}
			}
		}
	}
	currentOrderMutex.Lock()
	defer currentOrderMutex.Unlock()
	currentOrder = []Event{}
	for _, target := range currentState.data {
		currentOrder = append(currentOrder, target)
	}
	currentOrder = orderByRankings(orderByMentions(currentOrder))
	lastCalculated = time.Now()
}

const kvalue int64 = 400

func resetMentions(in map[mindmachine.S256Hash]Event) (out map[mindmachine.S256Hash]Event) {
	defer timeTrack(time.Now(), "resetMentions")
	out = make(map[mindmachine.S256Hash]Event)
	for hash, target := range in {
		target.Mentions = 0
		target.Score = kvalue
		out[hash] = target
	}
	return
}

func orderByRankings(in []Event) []Event {
	defer timeTrack(time.Now(), "orderByRankings")
	sort.SliceStable(in, func(i, j int) bool {
		return in[i].Score > in[j].Score
	})
	return in
}

func orderByMentions(in []Event) []Event {
	defer timeTrack(time.Now(), "orderByMentions")
	sort.Slice(in, func(i, j int) bool {
		return in[i].Mentions > in[j].Mentions
	})
	return in
}

func writeDb() {
	defer timeTrack(time.Now(), "writeDb")
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	b, err := json.MarshalIndent(currentState.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 1)
	} else {
		database.Write("eventbucket", "current", b)
	}
}
