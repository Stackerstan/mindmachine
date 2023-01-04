package patches

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	dircopy "github.com/otiai10/copy"
	"github.com/stackerstan/go-nostr"
	"mindmachine/mindmachine"
)

func (r *Repository) BuildTip() error {
	//todo: validate we are not applying patches that have not been merged
	//delete the current tip
	err := os.RemoveAll(r.rootDir() + "TIP")
	if err != nil {
		return err
	}
	err = os.MkdirAll(r.rootDir()+"TIP", 0777)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 1)
		return err
	}
	patchChain := r.getOrderedSliceOfPatches()
	for _, _patch := range patchChain {
		mindmachine.LogCLI("Applying patch "+fmt.Sprint(_patch.Height, ": ", _patch.UID), 4)
		err := r.applyPatch(_patch)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 1)
			return err
		}
	}
	return nil
}

//StartWork creates a new working directory based on the current tip of the patch chain. A Contributor can
//then start working on a Patch in this directory. It returns the full path to the working directory.
func (r *Repository) StartWork(problem mindmachine.S256Hash) (string, error) {
	//todo if we get spam: Check if this pubkey has been USH validated
	//if !identity.IsValid(bitswarm.MyWallet().Account) {
	//	return "", fmt.Errorf("your account has not been identified")
	//}
	//todo: Verify that this problem exists and has been claimed by this pubkey
	//(or not claimed by anyone, in which case mark it as claimed)
	location := r.offer(problem)
	if !isEmpty(location) {
		return "", fmt.Errorf("the directory " + location + " is not empty, I could just delete " +
			"the directory and reset, but it might contain your work so I'll leave that to you")
	}
	err := r.BuildTip()
	if err != nil {
		return "", err
	}
	//make a local backup of the current tip
	err = dircopy.Copy(r.tip(), r.offerBase(problem))
	if err != nil {
		return "", err
	}
	err = dircopy.Copy(r.tip(), r.offer(problem))
	if err != nil {
		return "", err
	}
	//Store our problem hash in the offer metadata
	var tempPatch Patch
	tempPatch.Problem = problem
	patchBytes, err := mindmachine.ToBytes(tempPatch)
	if err != nil {
		return "", err
	}
	_ = os.MkdirAll(r.offer(problem)+"/.mindmachine", 0777)
	err = ioutil.WriteFile(r.offer(problem)+"/.mindmachine/offer", patchBytes, 0777)
	if err != nil {
		return "", err
	}
	return r.offer(problem), nil
}

//CreateUnsignedPatchOffer takes the problemID and produces a patch. If the patch is larger than 100kb, it creates
//multiple shard Events, with the final Event containing an ordered list of IDs for all Events required to rebuild
//the full patch.
func (r *Repository) CreateUnsignedPatchOffer(problem mindmachine.S256Hash) (patch []nostr.Event, err error) {
	//Get the previous patch first
	//read in the current offerbase metadata so that we can get the hash of the TIP at the time the OFFERBASE was created
	//this is the patch that the new patch (this current patch) is based on, we need the hash of the base patch to create
	//the new patch.
	var baseHash mindmachine.S256Hash
	offerBasePatchBytes, err := os.ReadFile(r.offerBase(problem) + "/.mindmachine/patch")
	if err != nil {
		mindmachine.LogCLI(err, 1)
		return patch, err
	}
	var offerBasePatch Patch
	err = offerBasePatch.fromBytes(offerBasePatchBytes)
	if err != nil {
		mindmachine.LogCLI(err, 1)
		return patch, err
	}
	baseHash = offerBasePatch.UID
	mindmachine.LogCLI("Creating a patch. BasedOn: "+fmt.Sprint(baseHash), 4)

	//Build patch from OFFERBASE to OFFER (changes made by the Contributor)
	newPatch, err := r.diff(problem, r.offerBase(problem), r.offer(problem))
	if err != nil {
		mindmachine.LogCLI(err, 1)
		return patch, err
	}

	//Validate that this patch can be merged on current patch chain tip without conflicts
	err = r.validateNoConflicts(newPatch)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 1)
		return patch, err
	}
	newPatch.RepoName = r.Name
	newPatch.CreatedBy = mindmachine.MyWallet().Account
	newPatch.UID = mindmachine.Sha256(newPatch.Diff)
	newPatch.CreatedAt = mindmachine.CurrentState().Processing.Height
	newPatch.BasedOn = baseHash
	newPatch.Problem = problem
	//todo Validate that this Contributor still has the Claim on the problem, if not then warn them that they might
	//be called upon to meet with pistols at dawn to defend their honour.
	return patch2Events(newPatch)
}

func patch2Events(patch Patch) (events []nostr.Event, err error) {
	diffs := split(patch.Diff, 10*1024)
	var ptchs []Patch
	for _, diff := range diffs {
		p := patch
		p.Diff = diff
		ptchs = append(ptchs, p)
	}
	var sharded bool
	if len(ptchs) > 1 {
		sharded = true
	} else {
		sharded = false
	}
	for i, ptch := range ptchs {
		if e, err := patch2event(ptch, sharded, i); err != nil {
			return nil, err
		} else {
			events = append(events, e)
		}
	}
	if len(events) > 1 {
		//generate an event that combines all the patch shards
		c := Kind641002{
			RepoName: patch.RepoName,
			Problem:  patch.Problem,
			BasedOn:  patch.BasedOn,
			UID:      mindmachine.Sha256(patch.Diff),
		}
		for _, event := range events {
			c.Shards = append(c.Shards, event.ID)
		}
		j, err := json.Marshal(c)
		if err != nil {
			return events, err
		}
		e := nostr.Event{
			PubKey:    mindmachine.MyWallet().Account,
			CreatedAt: time.Now(),
			Kind:      641002,
			Tags:      nostr.Tags{},
			Content:   fmt.Sprintf("%s", j),
		}
		e.ID = e.GetID()
		events = append(events, e)
	}
	return
}

func split(buf []byte, lim int) [][]byte {
	var chunk []byte
	chunks := make([][]byte, 0, len(buf)/lim+1)
	for len(buf) >= lim {
		chunk, buf = buf[:lim], buf[lim:]
		chunks = append(chunks, chunk)
	}
	if len(buf) > 0 {
		chunks = append(chunks, buf[:len(buf)])
	}
	return chunks
}

func join(b [][]byte) []byte {
	var joined []byte
	for _, bytes := range b {
		joined = append(joined, bytes...)
	}
	return joined
}

//func event2patch(ev nostr.Event) (p Patch, e error) {
//	var unmarshalled Kind641002
//	err := json.Unmarshal([]byte(ev.Content), &unmarshalled)
//	if err != nil {
//		return p, err
//	}
//	if len(unmarshalled.Shards) > 0 {
//		return p, fmt.Errorf("this event combines multiple shards")
//	}
//	p.BasedOn = unmarshalled.BasedOn
//	p.Problem = unmarshalled.Problem
//	p.UID = unmarshalled.UID
//	p.CreatedBy = ev.PubKey
//	p.RepoName = unmarshalled.RepoName
//	p.Diff = []byte(unmarshalled.Diff)
//	return
//}

func patch2event(patch Patch, shard bool, shardNumber int) (event nostr.Event, err error) {
	kind := 641002
	if shard {
		kind = 641003
	}
	event.PubKey = mindmachine.MyWallet().Account
	event.Kind = kind
	event.CreatedAt = time.Now()
	event.Tags = nostr.Tags{}
	content := Kind641002{
		RepoName:    patch.RepoName,
		Problem:     patch.Problem,
		BasedOn:     patch.BasedOn,
		Diff:        fmt.Sprintf("%x", patch.Diff),
		ShardNumber: shardNumber,
		UID:         mindmachine.Sha256(patch.Diff),
	}
	j, err := json.Marshal(content)
	if err != nil {
		return event, err
	}
	event.Content = fmt.Sprintf("%s", j)
	event.ID = event.GetID()
	return event, nil
}

func (r *Repository) UnsignedMergePatchOffer(patchUID string) (n nostr.Event, e error) {
	var k Kind641004
	//if !identity.IsMaintainer(mindmachine.MyWallet().Account) {
	//	return n, fmt.Errorf("you are not a maintainer")
	//}
	p, ok := r.getPatch(patchUID)
	if !ok {
		return n, fmt.Errorf("could not find this patch in the database")
	}
	//Validate that this patch can be merged on current patch chain tip without conflicts
	err := r.validateNoConflicts(p)
	if err != nil {
		if !strings.Contains(err.Error(), "has conflicts") {
			return n, err
		}
		k.Conflicts = true
	}
	//AppendData current TIP height +1
	k.Height = r.getLatestPatch().Height + 1
	k.Sequence = p.Sequence + 1
	k.RepoName = r.Name
	k.UID = patchUID
	marshalled, err := json.Marshal(k)
	if err != nil {
		return n, err
	}
	n.Kind = 641004
	n.CreatedAt = time.Now()
	n.Content = fmt.Sprintf("%s", marshalled)
	return
}
