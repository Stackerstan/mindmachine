package sequence

import (
	"fmt"

	"github.com/sasha-s/go-deadlock"
	"mindmachine/mindmachine"
)

//GetSequence SHOULD be called when producing an event locally.
//it MUST NOT be used to validate the current sequence.
func GetSequence(account mindmachine.Account) int64 {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	if id, ok := currentState.data[account]; ok {
		return id.Sequence
	}
	return 0
}

var ls = &deadlock.Mutex{}

func LockSequence(account mindmachine.Account) int64 {
	ls.Lock()
	defer ls.Unlock()
	currentState.mutex.Lock()
	locked = account
	if cs, ok := currentState.data[account]; ok {
		return cs.Sequence
	} else {
		cs := Sequence{
			Account:  account,
			Sequence: 0,
		}
		currentState.data[account] = cs
	}
	return currentState.data[account].Sequence
}

var locked mindmachine.Account

func UnlockSequence(account mindmachine.Account, newSeq int64) error {
	defer currentState.mutex.Unlock()
	if account != locked {
		return fmt.Errorf("you requested to unlock " + account + "but we currently have " + locked + "locked instead")
	}
	if currentState.data[account].Sequence+1 == newSeq {
		cs := currentState.data[account]
		cs.Sequence++
		currentState.data[account] = cs
		currentState.takeSnapshot()
		return nil
	}
	return fmt.Errorf("failed to increment sequence")
}
