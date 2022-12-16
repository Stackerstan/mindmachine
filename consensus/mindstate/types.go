package mindstate

import (
	"github.com/fiatjaf/go-nostr"
	"github.com/sasha-s/go-deadlock"
	"mindmachine/mindmachine"
)

type db struct {
	data  map[mindmachine.S256Hash]VpssData
	mutex *deadlock.Mutex
}

type VpssData struct {
	Proofs          []nostr.Event
	Mind            string //the Mind that this vpss is targeting
	MindStateHash   mindmachine.S256Hash
	Sequence        int64
	Height          int64
	NailedTo        mindmachine.S256Hash //the Shares state that this state was nailed to - this is the current state of the shares when this Mind-state was created. If Mind is shares, it does not need to be nailed to anything.
	VpSigned        int64                //the amount of votepower that has signed this state
	VpAvailable     int64                //the amount of votepower available at the share state this state is nailed to.
	Permille        int64
	HaveNailedState bool
	HaveMindState   bool
	Sigchain        bool // is there a chain of signatures back to the ignition?
}

//Kind640001 STATUS:DRAFT
//Used for: producing and event to tell peers our current state and how we got here
type Kind640001 struct {
	Height      int64                `json:"height" status:"draft"`
	EventIDs    []string             `json:"events" status:"draft"`
	LatestState map[string]MindState `json:"vpss_state" status:"draft"`
	OpReturn    string               `json:"op_return" status:"draft"`
}

type MindState struct {
	Mind            string
	SigningAccounts []mindmachine.Account
	State           string
	Height          int64 `json:"height" status:"draft"`
	Sequence        int64 `json:"sequence" status:"draft"`
	Permille        int64 `json:"permille" status:"draft"`
}
