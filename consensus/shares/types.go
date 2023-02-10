package shares

import (
	"mindmachine/mindmachine"
)

type Share struct {
	LeadTimeLockedShares   int64
	LeadTime               int64 // in multiples of 2016 blocks (2 weeks)
	LastLtChange           int64 // btc height
	Expenses               []Expense
	LeadTimeUnlockedShares int64 //Approved Expenses can be swept into here and sold even if Participant's LT is >0
	OpReturnAddresses      []string
	Sequence               int64
}

type Expense struct {
	UID               string
	Problem           string //ID of problem in the issue tracker (optional)
	CommitMsg         string //The commit message from a merged patch on github or the patch chain. MUST NOT exceed 80 chars
	Solution          string //For now, this MUST be a sha256 hash of the diff that is merged on github, later this hash will be used in the patch chain.
	Amount            int64  //Satoshi
	RepaidAmount      int64
	WitnessedAt       int64 //Height at which we saw this Expense being created
	Nth               int64 //Expenses are repaid in the order they were approved, this is the 'height' of this expense, added upon approval
	Ratifiers         map[mindmachine.Account]struct{}
	RatifyPermille    int64
	Blackballers      map[mindmachine.Account]struct{}
	BlackballPermille int64
	Approved          bool
	SharesCreated     int64 //should be equal to the Amount in satoshi
}

//Kind640200 STATUS:DRAFT
//Used for adjusting lead time
type Kind640200 struct {
	AdjustLeadTime string //+ or -
	LockShares     int64
	UnlockShares   int64
	Sequence       int64
}

//Kind640202 STATUS:DRAFT
//Used for transferring Shares to another account
type Kind640202 struct {
	Amount    int64
	ToAccount string
	Note      string
	Sequence  int64
}

//Kind640204 STATUS:DRAFT
//Used for creating an Expense request
type Kind640204 struct {
	Problem   string //ID of problem from problem tracker (optional)
	CommitMsg string //<81 chars
	Solution  string //hash of diff
	Amount    int64  //amount being claimed in satoshi
	Sequence  int64
}

//Kind640206 STATUS:DRAFT
//Used for voting on an Expense request
//todo don't allow voting unless we are >500 Permille deep.
//Otherwise we have to vote again if the WitnessedAt height changes.
type Kind640206 struct {
	Account   string
	UID       string
	Ratify    bool
	Blackball bool
	Sequence  int64
}
