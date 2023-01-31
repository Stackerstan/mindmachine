package nostrkinds

import (
	"mindmachine/mindmachine"
)

type Kind struct {
	Kind        int64
	NIPs        []string //doki or urls
	App         string
	Description string //markdown
	Curator     mindmachine.Account
	LastUpdate  int64 //block height
	Sequence    int64
}

//Kind641800 STATUS:DRAFT
//Used for: modifying a Nostr kind
type Kind641800 struct {
	Kind          int64  `json:"kind" status:"draft"`
	NIP           string `json:"nip" status:"draft"`       //doki or url
	RemoveNIP     string `json:"removenip" status:"draft"` //string must exactly match an item in NIPs
	App           string `json:"app" status:"draft"`
	Description   string `json:"description" status:"draft"` //markdown
	RemoveCurator bool   `json:"remove_curator" status:"draft"`
	Sequence      int64  `json:"sequence" status:"draft"`
}
