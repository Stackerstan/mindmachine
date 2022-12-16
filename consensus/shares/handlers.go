package shares

import (
	"fmt"

	"mindmachine/consensus/identity"
	"mindmachine/mindmachine"
)

//todo update sequencenumbers
func HandleEvent(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	if sig, _ := event.CheckSignature(); !sig {
		return
	}
	if mind, _ := mindmachine.WhichMindForKind(event.Kind); mind == "shares" {
		if identity.IsUSH(event.PubKey) {
			currentState.mutex.Lock()
			defer currentState.mutex.Unlock()
			if _, ok := currentState.data[event.PubKey]; ok {
				switch event.Kind {
				case 640200:
					return handle640200(event)
				case 640202:
					return handle640202(event)
				case 640204:
					return handle640204(event)
				case 640206:
					return handle640206(event)
				}
			}
		}
	}
	return
}

func handle640206(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	var unmarshalled Kind640206
	err := json.Unmarshal([]byte(event.Content), &unmarshalled)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 3)
	} else {
		existing := currentState.data[event.PubKey]
		if existing.Sequence+1 == unmarshalled.Sequence {
			if VotePowerForAccount(event.PubKey) > 0 {
				if existingTarget, ok := currentState.data[unmarshalled.Account]; ok {
					for i, expens := range existingTarget.Expenses {
						if expens.UID == unmarshalled.UID {
							if unmarshalled.Blackball && !unmarshalled.Ratify {
								if _, ok := existingTarget.Expenses[i].Blackballers[event.PubKey]; !ok {
									existingTarget.Expenses[i].Blackballers[event.PubKey] = struct{}{}
									existingTarget.updateExpenses()
									currentState.data[unmarshalled.Account] = existingTarget
									existing.Sequence++
									currentState.data[event.PubKey] = existing
									return currentState.takeSnapshot(), true
								}
							}
							if unmarshalled.Ratify && !unmarshalled.Blackball {
								if _, ok := existingTarget.Expenses[i].Ratifiers[event.PubKey]; !ok {
									existingTarget.Expenses[i].Ratifiers[event.PubKey] = struct{}{}
									existingTarget.updateExpenses()
									currentState.data[unmarshalled.Account] = existingTarget
									existing.Sequence++
									currentState.data[event.PubKey] = existing
									return currentState.takeSnapshot(), true
								}
							}
						}
					}

				}
			}
		}
	}
	return
}

func handle640204(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	var unmarshalled Kind640204
	err := json.Unmarshal([]byte(event.Content), &unmarshalled)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 3)
	} else {
		existing := currentState.data[event.PubKey]
		if existing.Sequence+1 == unmarshalled.Sequence {
			if identity.IsUSH(event.PubKey) {
				expense := Expense{
					Problem:      unmarshalled.Problem,
					Solution:     unmarshalled.Solution,
					Amount:       unmarshalled.Amount,
					WitnessedAt:  mindmachine.CurrentState().Processing.Height,
					Ratifiers:    make(map[mindmachine.Account]struct{}),
					Blackballers: make(map[mindmachine.Account]struct{}),
					Approved:     false,
				}
				expense.UID = expense.generateUID()
				existing.Expenses = append(existing.Expenses, expense)
				existing.Sequence++
				currentState.data[event.PubKey] = existing
				return currentState.takeSnapshot(), true
			}
		}
	}
	return
}

func handle640202(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	var unmarshalled Kind640202
	err := json.Unmarshal([]byte(event.Content), &unmarshalled)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 3)
	} else {
		existing := currentState.data[event.PubKey]
		if existing.Sequence+1 == unmarshalled.Sequence {
			if unmarshalled.Amount <= existing.LeadTimeUnlockedShares && len(unmarshalled.ToAccount) == 64 {
				if receiverExisting, ok := currentState.data[unmarshalled.ToAccount]; ok {
					if identity.IsUSH(unmarshalled.ToAccount) {
						existing.LeadTimeUnlockedShares = existing.LeadTimeUnlockedShares - unmarshalled.Amount
						receiverExisting.LeadTimeUnlockedShares = receiverExisting.LeadTimeUnlockedShares + unmarshalled.Amount
						existing.Sequence++
						currentState.data[event.PubKey] = existing
						currentState.data[unmarshalled.ToAccount] = receiverExisting
						return currentState.takeSnapshot(), true
					}
				}
			}
		}
	}
	return
}

func handle640200(event mindmachine.Event) (h mindmachine.HashSeq, b bool) {
	var unmarshalled Kind640200
	err := json.Unmarshal([]byte(event.Content), &unmarshalled)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 3)
	} else {
		existing := currentState.data[event.PubKey]
		if existing.Sequence+1 == unmarshalled.Sequence {
			if existing.adjustLeadTime(unmarshalled.AdjustLeadTime) {
				currentState.data[event.PubKey] = existing
				return currentState.takeSnapshot(), true
			}
			if existing.LeadTimeUnlockedShares >= unmarshalled.LockShares && unmarshalled.LockShares > 0 {
				existing.LeadTimeUnlockedShares = existing.LeadTimeUnlockedShares - unmarshalled.LockShares
				existing.LeadTimeLockedShares = existing.LeadTimeLockedShares + unmarshalled.LockShares
				existing.Sequence++
				currentState.data[event.PubKey] = existing
				return currentState.takeSnapshot(), true
			}
			if existing.LeadTime == 0 {
				if existing.LeadTimeLockedShares >= unmarshalled.UnlockShares {
					existing.LeadTimeLockedShares = existing.LeadTimeLockedShares - unmarshalled.UnlockShares
					existing.LeadTimeUnlockedShares = existing.LeadTimeUnlockedShares + unmarshalled.UnlockShares
					existing.Sequence++
					currentState.data[event.PubKey] = existing
					return currentState.takeSnapshot(), true
				}
			}

		}
	}
	return
}

func (s *Share) updateExpenses() (b bool) {
	//todo run this once per block against ALL shares, not just when we receive votes
	for i, expens := range s.Expenses {
		//Add up the permille of blackballers and ratifiers
		if !expens.Approved {
			expens.BlackballPermille = 0
			for blackballer := range expens.Blackballers {
				if bbshare, ok := currentState.data[blackballer]; ok {
					expens.BlackballPermille += bbshare.Permille()
				}
			}
			expens.RatifyPermille = 0
			for ratifier := range expens.Ratifiers {
				if bbshare, ok := currentState.data[ratifier]; ok {
					expens.RatifyPermille += bbshare.Permille()
				}

			}
			//check blackball and ratifier numbers vs number of blocks since expense was created
			activePeriod := mindmachine.CurrentState().Processing.Height - expens.WitnessedAt
			// <Rule> An Expense MUST be Approved if it achieves a Ratification Rate of greater than 66.6%, and Blackball Rate of less than 6%, after an Active Period greater than 1,008 Blocks.
			if activePeriod > 1008 {
				if expens.BlackballPermille < 60 {
					if expens.RatifyPermille > 666 {
						s.Expenses[i].Approved = true
					}
				}
			}
			// <Rule> An Expense MUST be Approved if it achieves a Ratification Rate of greater than 50%, and Blackball Rate of no greater than 0% after an Active Period greater than 144 Blocks.
			if activePeriod > 144 {
				if expens.BlackballPermille == 0 {
					if expens.RatifyPermille > 500 {
						s.Expenses[i].Approved = true
					}
				}
			}
			// <Rule> An Expense MUST be Approved if it achieves a Ratification Rate of greater than 90% and a Blackball Rate of no greater than 0% after an Active Period greater than 0 Blocks.
			if activePeriod > 0 {
				if expens.BlackballPermille == 0 {
					if expens.RatifyPermille > 900 {
						s.Expenses[i].Approved = true
					}
				}
			}
			//if approved, sweep into leadtimeunlocked shares
			if expens.Approved {
				s.LeadTimeUnlockedShares += expens.Amount
				b = true
			}
		}
	}
	return
}

func (s *Share) adjustLeadTime(option string) bool {
	if s.LastLtChange+2016 <= mindmachine.CurrentState().Processing.Height {
		if option == "+" {
			s.LeadTime++
			return true
		}
		if option == "-" {
			s.LeadTime--
			return true
		}
	}
	return false
}

func (s *Share) checkSequence(seq int64) bool {
	if s.Sequence+1 == seq {
		return true
	}
	return false
}

func (e *Expense) generateUID() string {
	return mindmachine.Sha256(fmt.Sprintf(e.Problem, e.Solution, e.Amount, e.WitnessedAt))
}
