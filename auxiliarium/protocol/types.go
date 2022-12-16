package protocol

import (
	"mindmachine/mindmachine"
)

type Item struct {
	UID          mindmachine.S256Hash // Initially calculated from Item.Text string + random nonce, never changed
	CreatedBy    mindmachine.Account
	WitnessedAt  int64 // BTC height when this was created. Set by the welay, the block that the welay is processing when it recieves the event cretaing this. The evidence of it existing at that time is that the Event is in the messagepacker at that height and the messagepacker hash is in the merkle tree which goes into op_return.
	Sequence     int64
	LastUpdate   int64                            // BTC height when this was last updated
	Problem      mindmachine.S256Hash             // All rules must be in response to a problem
	Text         mindmachine.S256Hash             // pointer to a comprendo with Unformatted text explaining the rule, max length 280 characters
	Kind         int64                            // Definition|Goal|Rule|Invariant|Protocol
	Nests        []mindmachine.S256Hash           // Ordered list of sub-Items
	Ratifiers    map[mindmachine.Account]struct{} //List of accounts that have ratified this Item
	Blackballers map[mindmachine.Account]struct{}
	ApprovedAt   int64                // BTC height when this rule reached 1000 Permille of votepower
	Supersedes   mindmachine.S256Hash // If this item replaces an existing rule
	SupersededBy mindmachine.S256Hash // If this item has been replaced
	Parent       mindmachine.S256Hash
}

const (
	Definition int64 = 1
	Goal       int64 = 2
	Rule       int64 = 3
	Invariant  int64 = 4
	Protocol   int64 = 5
)

//Kind640600 STATUS:DRAFT
//Used for: creating a new Protocol item
//todo ask contributors to update status to x when they use this spec in a frontend
type Kind640600 struct {
	Problem    string   `json:"problem" status:"draft"`
	Text       string   `json:"text" status:"draft"`
	Kind       string   `json:"kind" status:"draft"`
	Supersedes string   `json:"supersedes" status:"draft"`
	Nests      []string `json:"nests" status:"draft"`
	Parent     string   `json:"parent" status:"draft"`
}

//Kind640602 STATUS:DRAFT
//Used for: voting on a Protocol item
type Kind640602 struct {
	Target    string `json:"target" status:"draft"`
	Ratify    bool   `json:"ratify" status:"draft"`
	Blackball bool   `json:"blackball" status:"draft"`
	Sequence  int64  `json:"sequence" status:"draft"`
}

//Kind640604 STATUS:DRAFT
//Used for: updating the order of Nested Item(s) under an Item
type Kind640604 struct {
	Target   string   `json:"target" status:"draft"`
	Nests    []string `json:"nests" status:"draft"`
	Sequence int64    `json:"sequence" status:"draft"`
}
