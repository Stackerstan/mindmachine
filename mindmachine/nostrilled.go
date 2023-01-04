package mindmachine

import (
	"strconv"
	"strings"
	"time"

	"github.com/stackerstan/go-nostr"
)

type Event struct {
	ID          string
	PubKey      string
	CreatedAt   time.Time
	Kind        int64
	Tags        nostr.Tags
	Content     string
	Sig         string
	Commands    Instructions
	WitnessedAt int64 //the height when we first witnessed this event
}

type Instructions struct {
	List     []string
	Mind     string
	Sequence int64
}

var minds = make(map[string]string)

//RegisterMind registers a Mind's name and event kinds so that we can route events to the right consumer.
func RegisterMind(kinds []int64, word, mind string) bool {
	if _, taken := minds[word]; !taken {
		minds[word] = mind
		if err := registerKinds(kinds, mind); err != nil {
			return false
		}
		return true
	}
	return false
}

func (e *Event) parseInstructions() (ins Instructions) {
	if indx := strings.Index(e.Content, "mindmachine ~$ "); indx != -1 {
		commands := strings.Fields(e.Content[indx+9:])
		for _, command := range commands {
			ins.List = append(ins.List, command)
			if mind, ok := minds[command]; ok {
				ins.Mind = mind
			}
		}
		for i, command := range ins.List {
			if command == "sequence" {
				seq, err := strconv.ParseInt(ins.List[i+1], 10, 64)
				if err != nil {
					LogCLI(err.Error(), 3)
				} else {
					ins.Sequence = seq
				}
			}
		}
	}
	return
}

// Height returns the height that this event was created at, if no height is available it returns false.
func (e *Event) Height() (int64, bool) {
	for _, tag := range e.Tags {
		//fmt.Printf("%#v", e.Tags)
		if len(tag) > 0 {
			if tag[0] == "height" {
				i, err := strconv.ParseInt(tag[1], 10, 64)
				if err == nil {
					return i, true
				}
			}
		}
	}
	for _, tag := range e.Tags {
		if len(tag) > 0 {
			if tag[0] == "block" {
				i, err := strconv.ParseInt(tag[1], 10, 64)
				if err == nil {
					return i, true
				}
			}
		}
	}
	return 0, false
}

//GetSingleTag returns the value of the first tag that matches t string.
func (e *Event) GetSingleTag(t string) (value string, ok bool) {
	for _, tag := range e.Tags {
		if len(tag) > 0 {
			if tag[0] == t {
				if len(tag[1]) > 0 {
					return tag[1], true
				}
			}
		}
	}
	if v := e.getSingleCommand(t); len(v) > 0 {
		return v, true
	}
	return
}

func (e *Event) GetTags(t string) (value []string, ok bool) {
	for _, tag := range e.Tags {
		if tag[0] == t {
			ok = true
			for _, s := range tag {
				if s != t {
					value = append(value, s)
				}
			}
		}
	}
	if len(value) == 0 {
		if v := e.getSingleCommand(t); len(v) > 0 {
			ok = true
			value = strings.Split(v, ";")
		}
	}
	return
}

func (e *Event) Sequence() int64 {
	if seq, ok := e.GetSingleTag("sequence"); ok {
		if s, err := strconv.ParseInt(seq, 10, 64); err == nil {
			return s
		}
	}
	return 0
}

// Dataset returns the name of the Dataset (within a Mind) that this Event is applicable to.
func (e *Event) Dataset() (s string) {
	for _, tag := range e.Tags {
		if tag[0] == "dataset" {
			if len(tag[1]) > 0 {
				return tag[1]
			}
		}
	}
	return
}

func (e *Event) Target() (t S256Hash, ok bool) {
	var trgt S256Hash
	for _, tag := range e.Tags {
		//todo handle nipxx properly (the one that specifies the order of e tags)
		if tag[0] == "target" || tag[0] == "e" {
			if len(tag[1]) == 64 {
				trgt = tag[1]
				return trgt, true
			}
		}
	}
	if len(trgt) != 64 {
		trgt = e.getSingleCommand("target")
	}
	if len(trgt) == 64 {
		return trgt, true
	}

	return
}

func (e *Event) getSingleCommand(commandName string) (s string) {
	for i, command := range e.Commands.List {
		if command == commandName {
			if len(e.Commands.List[i+1]) > 0 {
				return e.Commands.List[i+1]
			}
		}
	}
	return
}

func (e *Event) getCommands(commandName string) (s []string) {
	for i, command := range e.Commands.List {
		if command == commandName {
			if len(e.Commands.List[i+1]) > 0 {
				s = append(s, e.Commands.List[i+1])
			}
		}
	}
	return
}

func (e *Event) CheckSignature() (bool, error) {
	return e.convertToNostrEvent().CheckSignature()
}

func (e *Event) convertToNostrEvent() nostr.Event {
	return nostr.Event{
		ID:        e.ID,
		PubKey:    e.PubKey,
		CreatedAt: e.CreatedAt,
		Kind:      int(e.Kind),
		Tags:      e.Tags,
		Content:   e.Content,
		Sig:       e.Sig,
	}
}

func (e *Event) Nostr() nostr.Event {
	return e.convertToNostrEvent()
}

//ConvertToInternalEvent parses a nostr event and converts it to a locally Typed event
func ConvertToInternalEvent(evt *nostr.Event) Event {
	e := Event{
		ID:        evt.ID,
		PubKey:    evt.PubKey,
		CreatedAt: evt.CreatedAt,
		Kind:      int64(evt.Kind),
		Tags:      evt.Tags,
		Content:   evt.Content,
		Sig:       evt.Sig,
	}
	e.Commands = e.parseInstructions()
	return e
}

func (e *Event) ContainsCommand(command string) bool {
	for _, a := range e.Commands.List {
		if a == command {
			return true
		}
	}
	return false

}
