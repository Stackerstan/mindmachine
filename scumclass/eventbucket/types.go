package eventbucket

import (
	"github.com/stackerstan/go-nostr"
)

type Event struct {
	EventID    string
	Event      nostr.Event
	Kind       int64
	Score      int64
	Mentions   int64
	MentionMap map[string]struct{}
}

type Kind struct {
	Kind  int64
	Count int64
}

//1. get ALL kind 1 events
//2. get ALL mentions of the kind 1 events
//3. sort by most mentions
