package identity

import (
	"mindmachine/mindmachine"
)

type Identity struct {
	Account               mindmachine.Account
	Name                  string
	About                 string
	Picture               []byte
	LegacyIdentities      []LegacySocialMediaIdentity
	UniqueSovereignBy     mindmachine.Account // the account that has validated this account is a real human and doesn't already have an account (identity chain)
	CharacterVouchedForBy map[mindmachine.Account]struct{}
	MaintainerBy          mindmachine.Account // the maintainer who has made this account a maintainer (maintainer chain)
	Sequence              int64
	Pubkeys               []string
	OpReturnAddr          [][]string
	Order                 int64
}

type LegacySocialMediaIdentity struct {
	Platform    string //e.g. Facebook
	Username    string
	EvidenceURL string //link to a post (or whatever) containing proof that this Participant controls the legacy platform account
}

type Kind0 struct {
	Name        string `json:"name"`
	About       string `json:"about"`
	DisplayName string `json:"display_name"`
}

//Evolution of public contracts: we can experiment behind this API by keeping Kind640400 (a Nostr Kind that is hereby
//defined by it's JSON unmarshalling struct) and adding new things like DisplayName or KittenNames. Any new things
//MUST be optional once this progresses past DRAFT status, if we need to add mandatory items under this Kind we need a new Kind.

//Kind640400 STATUS:DRAFT
//Used for Identity.Name Identity.About
type Kind640400 struct {
	Name     string `json:"name" status:"draft"`
	About    string `json:"about"`
	Sequence int64  `json:"sequence"`
}

//Kind640402 STATUS:DRAFT
//Used for adding Participants to the USH and Maintainer tree.
type Kind640402 struct {
	Target     string `json:"target"`
	Maintainer bool   `json:"maintainer"`
	USH        bool   `json:"ush"`
	Character  bool   `json:"character"`
	Sequence   int64  `json:"sequence"`
}

//Kind640404 STATUS:DRAFT
//Used for adding Identity evidence if the Participant wants to dox themselves
type Kind640404 struct {
	Platform string `json:"platform"`
	Username string `json:"username"`
	Evidence string `json:"evidence"`
	Sequence int64  `json:"sequence"`
}

//Kind640406 STATUS:DRAFT
//Used for adding an OP_RETURN address
type Kind640406 struct {
	Address  string
	Proof    string //Bitcoin signed message of the user's pubkey
	Sequence int64  `json:"sequence"`
}
