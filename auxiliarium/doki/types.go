package doki

import (
	"mindmachine/mindmachine"
)

type Document struct {
	UID           string
	CreatedBy     mindmachine.Account
	GoalOrProblem string  //The text of the goal of this document or the problem this document is solving
	Patches       []Patch //ordered list of merged patches (event IDs)
	CurrentTip    string  //the current tip when we apply all the Patches
	MergedBy      string  //the maintainer who merged this document
	Sequence      int64
}

type Patch struct {
	EventID        mindmachine.S256Hash
	Patch          string
	CreatedBy      mindmachine.Account
	MergedBy       mindmachine.Account
	Problem        string
	RejectedBy     mindmachine.Account
	RejectedReason string
}

//Kind641200 STATUS:DRAFT
//Used for creating a new Document
type Kind641200 struct {
	GoalOrProblem string `json:"goal_or_problem"` //Max 280 chars
}

//Kind641202 STATUS:DRAFT
//Used for creating a patch to an existing document
type Kind641202 struct {
	DocumentUID string `json:"document_uid"` //UID of the Document
	Patch       string `json:"patch"`        // GNU Patch format
	Problem     string `json:"problem"`      //<280 characters explaining the problem (optional)
	Sequence    int64  `json:"sequence"`
}

//Kind641204 STATUS:DRAFT
//Used for merging a patch into a document or deleting it
//If the document itself has not been merged, it becomes merged with the first patch merge.
type Kind641204 struct {
	DocumentUID  string `json:"document_uid"`
	PatchEventID string `json:"patch_uid"`
	Operation    int64  //1 = delete, 2 = merge
	Sequence     int64
	Reason       string //plain text explaining reason for deleting (if we are deleting it)
}
