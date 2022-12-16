/*
Package patches is a General Service Mind responsible for administering the Patch Chain for each of our Repositories.
*/

package patches

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"syscall"

	"github.com/sasha-s/go-deadlock"
	"mindmachine/consensus/identity"
	"mindmachine/database"
	"mindmachine/mindmachine"
)

type db struct {
	data  map[string]*Repository
	mutex *deadlock.Mutex
}

var currentState = db{
	data:  make(map[mindmachine.S256Hash]*Repository),
	mutex: &deadlock.Mutex{},
}

// StartDb starts the database for this mind (the Mind-state). It blocks until the database is ready to use.
func StartDb(terminate chan struct{}, wg *sync.WaitGroup) {
	if !mindmachine.RegisterMind([]int64{641000, 641002, 641004}, "patches", "patches") {
		mindmachine.LogCLI("Could not register Patches Mind", 0)
	}
	// For some reason files are being saved with incorrect permissions, this seems to resolve it.
	syscall.Umask(000)
	// we need a channel to listen for a successful database start
	ready := make(chan struct{})
	// now we can start the database in a new goroutine
	go startDb(terminate, wg, ready)
	// when the database has started, the goroutine will close the `ready` channel.
	<-ready //This channel listener blocks until closed by `startDb`.
	mindmachine.LogCLI("Patches Mind has started", 4)
}

// startDb opens the database from disk (or creates it). It closes the `ready` channel once the database is ready to
// handle queries, and shuts down safely when the terminate channel is closed. Any upstream functions that need to
// know when the database has been shut down should wait on the provided waitgroup.
func startDb(terminate chan struct{}, wg *sync.WaitGroup, ready chan struct{}) {
	// We add a delta to the provided waitgroup so that upstream knows when when we've safely closed the database
	wg.Add(1)
	c, ok := database.Open("patches", "current")
	if ok {
		currentState.restoreFromDisk(c)
	}
	close(ready)
	// The database has been started. Now we wait on the terminate channel
	// until upstream closes it (telling us to shut down).
	<-terminate
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	b, err := json.MarshalIndent(currentState.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	database.Write("patches", "current", b)
	//Tell upstream that we have finished shutting down the databases
	wg.Done()
	mindmachine.LogCLI("Patches Mind has shut down", 4)
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
	for _, repository := range s.data {
		repository.mutex = &deadlock.Mutex{}
	}
}

func AllRepositories() map[string]*Repository {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	return currentState.data
}

func (r *Repository) GetMapOfPatches() map[mindmachine.S256Hash]Patch {
	m := make(map[mindmachine.S256Hash]Patch)
	for hash, patch := range currentState.data[r.Name].Data {
		m[hash] = patch
	}
	return m
}

func (r *Repository) getPatch(hash mindmachine.S256Hash) (Patch, bool) {
	patch, ok := currentState.data[r.Name].Data[hash]
	if ok {
		return patch, true
	}
	return Patch{}, false
}

func (r *Repository) getNextPatch(currentHeight int64) (p Patch, err error) {
	for _, this := range r.Data {
		if identity.IsMaintainer(this.Maintainer) {
			if currentHeight+1 == this.Height {
				return this, nil
			}
		}
	}
	return p, fmt.Errorf("requested height is tip")
}

func (r *Repository) getOrderedSliceOfPatches() []Patch {
	var currentHeight int64 = -1
	var patchChain []Patch
	for {
		patch, err := r.getNextPatch(currentHeight)
		if err != nil {
			if err.Error() == "requested height is tip" {
				return patchChain
			}
			return patchChain
		}
		currentHeight++
		patchChain = append(patchChain, patch)
	}
}

func (r *Repository) getLatestPatch() (p Patch) {
	patchSlice := r.getOrderedSliceOfPatches()
	if len(patchSlice) > 0 {
		return patchSlice[len(patchSlice)-1]
	}
	return
}

func (r *Repository) upsert(patch Patch) (hs mindmachine.HashSeq, err error) {
	if r.Name != patch.RepoName {
		return hs, fmt.Errorf("you sent a patch for " + patch.RepoName + " but this repo is " + r.Name)
	}
	r.Data[patch.UID] = patch
	b, err := json.MarshalIndent(currentState.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	database.Write("patches", "current", b)
	return latestHashSeq(), nil
}

func latestHashSeq() mindmachine.HashSeq {
	var sortedRepoNames []string
	for repoName := range currentState.data {
		sortedRepoNames = append(sortedRepoNames, repoName)
	}
	sort.Slice(sortedRepoNames, func(i, j int) bool {
		return sortedRepoNames[i] > sortedRepoNames[j]
	})
	var seq int64
	for _, repository := range currentState.data {
		repository.lock()
	}
	buf := bytes.Buffer{}
	pmap := make(map[string]Patch)
	for _, repoName := range sortedRepoNames {
		var sortedPatchUID []string
		for patchUID, p := range currentState.data[repoName].Data {
			sortedPatchUID = append(sortedPatchUID, patchUID)
			seq = seq + p.Sequence
			pmap[patchUID] = p
		}
		sort.Slice(sortedPatchUID, func(i, j int) bool {
			return sortedPatchUID[i] > sortedPatchUID[j]
		})

		for _, patchUID := range sortedPatchUID {
			p := pmap[patchUID]
			buf.WriteString(patchUID)
			buf.WriteString(fmt.Sprintf("%d", p.Height))
			buf.WriteString(p.Problem)
			buf.WriteString(p.Maintainer)
			buf.WriteString(p.BasedOn)
			buf.WriteString(p.CreatedBy)
			buf.WriteString(fmt.Sprintf("%d", p.CreatedAt))
			if p.Conflicts {
				buf.WriteByte(1)
			} else {
				buf.WriteByte(0)
			}
		}
	}
	for _, repository := range currentState.data {
		repository.unlock()
	}
	return mindmachine.HashSeq{
		Hash:      mindmachine.Sha256(buf.Bytes()),
		Sequence:  seq,
		Mind:      "patches",
		CreatedAt: mindmachine.CurrentState().Processing.Height,
	}
}
