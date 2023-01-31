package conductor

import (
	"fmt"

	"github.com/sasha-s/go-deadlock"
	"mindmachine/auxiliarium/doki"
	"mindmachine/auxiliarium/patches"
	"mindmachine/auxiliarium/problems"
	"mindmachine/auxiliarium/protocol"
	"mindmachine/consensus/identity"
	"mindmachine/consensus/mindstate"
	"mindmachine/consensus/sequence"
	"mindmachine/consensus/shares"
	"mindmachine/mindmachine"
	"mindmachine/scumclass/nostrkinds"
)

var handlerMutex = &deadlock.Mutex{}
var bloom = mindmachine.MakeNewInverseBloomFilter(10000)

// HandleMessage is the entry point for all messages into the conductor
func HandleMessage(e mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	<-ready
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	es := e.Sequence()
	cs := sequence.LockSequence(e.PubKey)
	if es == cs+1 {
		if bloom(e) {
			if hs, ok := handleEvent(e); ok {
				sequence.UnlockSequence(e.PubKey, e.Sequence())
				return hs, ok
			}
		}
	} else {
		mindmachine.LogCLI(fmt.Sprintf("invalid sequence number on event %s, current sequence is %d", e.ID, cs), 3)
	}
	sequence.UnlockSequence(e.PubKey, 0)
	return h, false
}

func handleEvent(e mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	mind, ok := mindmachine.WhichMindForKind(e.Kind)
	if ok {
		switch mind {
		case "problems":
			return problems.HandleEvent(e)
		case "identity":
			return identity.HandleEvent(e)
		case "protocol":
			return protocol.HandleEvent(e)
		case "patches":
			return patches.HandleEvent(e)
		case "vpss":
			err, vOk := mindstate.HandleVPSS(e)
			if err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			return h, vOk
		case "shares":
			return shares.HandleEvent(e)
		case "doki":
			return doki.HandleEvent(e)
		case "nostrkinds":
			return nostrkinds.HandleEvent(e)
		default:
			//fmt.Printf("\n%#v\n", e)
			//mindmachine.LogCLI("invalid event", 2)
			return
		}
	}
	mindmachine.LogCLI("this shouldn't happen", 0)
	return
}
