package problems

import (
	"mindmachine/mindmachine"
)

type Problem struct {
	UID         mindmachine.S256Hash // Initially calculated from Item.Text string + random nonce, never changed
	CreatedBy   mindmachine.Account
	WitnessedAt int64 // BTC height when this was created. Set by the welay, the block that the welay is processing when it recieves the event cretaing this. The evidence of it existing at that time is that the Event is in the messagepacker at that height and the messagepacker hash is in the merkle tree which goes into op_return.
	Sequence    int64
	LastUpdate  int64                // BTC height when this was last updated
	Parent      mindmachine.S256Hash // optional
	Title       mindmachine.S256Hash //pointer to a comprendo with Unformatted text describing the problem in less than 280 characters
	Description mindmachine.S256Hash //optional pointer to a comprendo with Text+ (mindmachine flavoured Markdown) for further details about the problem
	Closed      bool
	Curator     mindmachine.Account
	ClaimedBy   mindmachine.Account
	ClaimedAt   int64 //BTC height when this was last claimed
	Children    []mindmachine.S256Hash
}

//Kind640800 STATUS:DRAFT
//Used for: logging a new Problem
type Kind640800 struct {
	Title       string `json:"title" status:"draft"`       //Problem.Title
	Description string `json:"description" status:"draft"` //Problem.Description (can be markdown or plain text) (optional)
	Parent      string `json:"parent" status:"draft"`      //Problem.Parent (optional)
}

//Kind640802 STATUS:DRAFT
//Used for: modifying an existing Problem
type Kind640802 struct {
	Target        string `json:"target" status:"draft"`
	Title         string `json:"title" status:"draft"`       //Problem.Title
	Description   string `json:"description" status:"draft"` //Problem.Description (can be markdown or plain text) (optional)
	Parent        string `json:"parent" status:"draft"`      //Problem.Parent (optional)
	RemoveParent  bool   `json:"remove_parent" status:"draft"`
	Claim         bool   `json:"claim" status:"draft"`
	RemoveClaim   bool   `json:"remove_claim" status:"draft"`
	Close         bool   `json:"close" status:"draft"`
	ReOpen        bool   `json:"reopen" status:"draft"`
	Curate        bool   `json:"curate" status:"draft"`
	RemoveCurator bool   `json:"remove_curator" status:"draft"`
	Sequence      int64  `json:"sequence" status:"draft"`
}
