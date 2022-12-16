package patches

import (
	"bytes"
	"encoding/gob"

	"github.com/sasha-s/go-deadlock"

	"mindmachine/mindmachine"
)

type Repository struct {
	Name   string
	Data   map[mindmachine.S256Hash]Patch
	Ignore []string
	mutex  *deadlock.Mutex
}

//Kind641002 STATUS:DRAFT
//Used to create a new Patch
type Kind641002 struct {
	RepoName    string   //The name of the repository that this Patch is applicable to
	Problem     string   //The problem that this patch is solving
	BasedOn     string   //The patch that this patch was based on (diff'd against)
	Diff        string   //The patch itself
	UID         string   //A SHA256 digest of the raw patch
	Shards      []string //A slice of Event IDs for the shards that make up this patch (optional)
	ShardNumber int      `json:"shard_number"` //Only applicable to Kind641003
}

//Kind641003 STATUS:DRAFT
//Used to create a new shard to be included in a Kind641002 event
type Kind641003 Kind641002

type Patch struct {
	RepoName   string
	CreatedBy  mindmachine.Account
	Diff       []byte
	UID        mindmachine.S256Hash // hash of Diff
	Problem    mindmachine.S256Hash // the hash of the problem from the problem tracker
	Maintainer mindmachine.Account  // the account that merged this patch
	BasedOn    mindmachine.S256Hash // the patch that this patch is based on
	Height     int64                // height of this patch in the patch chain
	CreatedAt  int64                // BTC height when this patch was created
	Conflicts  bool
	Sequence   int64
}

func (p *Patch) fromBytes(patchBytes []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(patchBytes))
	err := dec.Decode(&p)
	if err != nil {
		return err
	}
	return nil
}

//Kind641000 STATUS:DRAFT
//Used to create a new repository
type Kind641000 struct {
	Problem  string `json:"problem"`
	RepoName string `json:"name"`
}

//Kind641004 STATUS:DRAFT
//Used to Merge a patch or report conflicts
type Kind641004 struct {
	RepoName  string
	UID       string
	Conflicts bool  `json:"conflicts"`
	Height    int64 `json:"height"`
	Sequence  int64 `json:"sequence"`
}
