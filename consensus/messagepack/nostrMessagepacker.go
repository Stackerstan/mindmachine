/*
Package messagepack is a Scum-class Mind intended to archive any Directives that caused any Mind running on this Welay
instance to change their Mind-state. When coupled with the VPSS Mind, this can be used to rebuild the Mind-state of
all Minds from scratch such the this Welay instance reaches Consensus with the rest of the Mindmachine network.
*/

package messagepack

import (
	"fmt"
	"strconv"

	"github.com/sasha-s/go-deadlock"
	"github.com/stackerstan/go-nostr"
	"mindmachine/database"
	"mindmachine/mindmachine"
)

type messagepack struct {
	Height   int64
	Messages []string
	sealed   bool
	mutex    *deadlock.Mutex
}

var mut = &deadlock.Mutex{}
var current messagepack

func packEvent(e nostr.Event) {
	mut.Lock()
	defer mut.Unlock()
	if current.Height == 0 {
		mindmachine.LogCLI("this should not happen", 0)
	}
	current.mutex.Lock()
	defer current.mutex.Unlock()
	if e.Kind == 640000 {
		//fmt.Printf("\n39\n%#v\n\n", e)
	}
	mind, ok := mindmachine.WhichMindForKind(int64(e.Kind))
	if ok {
		if mind != "samizdat" {
			current.Messages = append(current.Messages, e.ID)
		}
	}
	AddRequired(e)
}

func StartBlock(block mindmachine.Event) {
	mut.Lock()
	defer mut.Unlock()
	if current.Height != 0 {
		current.mutex.Lock()
	}
	if current.Height != 0 && !current.sealed {
		mindmachine.LogCLI("this should not happen", 0)
	}
	content, ok := block.GetTags("block")
	if !ok {
		fmt.Println(block)
		mindmachine.LogCLI("this should not happen", 0)
	}
	h, err := strconv.ParseInt(content[0], 10, 64)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	if h != current.Height+1 && current.Height != 0 {
		fmt.Printf("\ncurrent %d\nevent %d\n", current.Height, h)
		mindmachine.LogCLI("wrong height!", 0)
	}
	mindmachine.LogCLI("Initialising the eventpacker at "+fmt.Sprintf("%d", h), 4)
	m, ok := getMessagePack(h)
	if ok {
		m.mutex = &deadlock.Mutex{}
		m.sealed = false
		current = m
	} else {
		// does gc automagically free the memory here or are we leaving dangleberries?
		current = messagepack{
			Height:   h,
			Messages: []string{fmt.Sprintf("%d", h)},
			mutex:    &deadlock.Mutex{},
			sealed:   false,
		}
	}
}

//SealBlock writes the current state to disk and returns the number of messages written.
func SealBlock(height int64) int64 {
	mut.Lock()
	defer mut.Unlock()
	current.mutex.Lock()
	defer current.mutex.Unlock()
	if current.Height != height {
		mindmachine.LogCLI("this should not happen", 0)
	}

	//write data to disk

	database.Write("messagepack", fmt.Sprintf("%d", height), marshal(&current))
	current.sealed = true
	return int64(len(current.Messages))
}
