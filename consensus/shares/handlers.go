package shares

import (
	"fmt"

	"mindmachine/consensus/identity"
	"mindmachine/mindmachine"
)

func NewBlock() bool {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	for account, share := range currentState.data {
		currentState.data[account] = updateExpenses(share)
	}
	return validateInvariants()
}

func validateInvariants() bool {
	var totalApprovedExpenses int64
	var totalShares int64
	for _, share := range currentState.data {
		totalShares += share.LeadTimeUnlockedShares
		totalShares += share.LeadTimeLockedShares
		for _, expens := range share.Expenses {
			if expens.Approved {
				totalApprovedExpenses += expens.Amount
			}
		}
	}
	if totalShares-1 == totalApprovedExpenses {
		return true
	}
	mindmachine.LogCLI("Invalid number of shares!", 1)
	return false
}

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
		if currentState.data[event.PubKey].Sequence+1 == unmarshalled.Sequence {
			if votePowerForAccount(event.PubKey) > 0 {
				if target, ok := currentState.data[unmarshalled.Account]; ok {
					for i, expens := range target.Expenses {
						if expens.UID == unmarshalled.UID {
							if _, exists := target.Expenses[i].Blackballers[event.PubKey]; !exists {
								if _, exists := target.Expenses[i].Ratifiers[event.PubKey]; !exists {
									if unmarshalled.Blackball && !unmarshalled.Ratify {
										target.Expenses[i].Blackballers[event.PubKey] = struct{}{}
										t := updateExpenses(target)
										currentState.data[unmarshalled.Account] = t
										voter := currentState.data[event.PubKey]
										voter.Sequence++
										currentState.data[event.PubKey] = voter
										return currentState.takeSnapshot(), true
									}
									if _, ok := target.Expenses[i].Ratifiers[event.PubKey]; !ok {
										target.Expenses[i].Ratifiers[event.PubKey] = struct{}{}
										t := updateExpenses(target)
										currentState.data[unmarshalled.Account] = t
										voter := currentState.data[event.PubKey]
										voter.Sequence++
										currentState.data[event.PubKey] = voter
										return currentState.takeSnapshot(), true
									}
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
		existing, ok := currentState.data[event.PubKey]
		if !ok {
			existing = Share{
				LeadTimeLockedShares:   0,
				LeadTime:               0,
				LastLtChange:           mindmachine.MakeOrGetConfig().GetInt64("ignitionHeight"),
				Expenses:               []Expense{},
				LeadTimeUnlockedShares: 0,
				OpReturnAddresses:      []string{},
				Sequence:               0,
			}
		}
		if existing.Sequence+1 == unmarshalled.Sequence {
			if identity.IsUSH(event.PubKey) {
				expense := Expense{
					Problem:       unmarshalled.Problem,
					CommitMsg:     unmarshalled.CommitMsg,
					Solution:      unmarshalled.Solution,
					Amount:        unmarshalled.Amount,
					PullRequest:   unmarshalled.PullRequest,
					WitnessedAt:   mindmachine.CurrentState().Processing.Height,
					Ratifiers:     make(map[mindmachine.Account]struct{}),
					Blackballers:  make(map[mindmachine.Account]struct{}),
					Approved:      false,
					SharesCreated: 0,
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

func updateExpenses(s Share) Share {
	for i, expens := range s.Expenses {
		//Add up the permille of blackballers and ratifiers
		if !expens.Approved && !expens.Rejected {
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
						expens.Approved = true
					}
				}
			}
			// <Rule> An Expense MUST be Approved if it achieves a Ratification Rate of greater than 50%, and Blackball Rate of no greater than 0% after an Active Period greater than 144 Blocks.
			if activePeriod > 144 {
				if expens.BlackballPermille == 0 {
					if expens.RatifyPermille > 500 {
						expens.Approved = true
					}
				}
			}
			// <Rule> An Expense MUST be Approved if it achieves a Ratification Rate of greater than 90% and a Blackball Rate of no greater than 0% after an Active Period greater than 0 Blocks.
			if activePeriod > 0 {
				if expens.BlackballPermille == 0 {
					if expens.RatifyPermille > 900 {
						expens.Approved = true
					}
				}
			}

			if expens.BlackballPermille == 0 {
				if expens.RatifyPermille == 1000 {
					expens.Approved = true
				}
			}

			if expens.BlackballPermille > 100 {
				expens.Rejected = true
			}
			//if approved, sweep into leadtimeunlocked shares
			if expens.Approved {
				s.LeadTimeUnlockedShares += expens.Amount
				expens.SharesCreated = expens.Amount
				expens.Nth = getNth()
				//b = true
			}
			s.Expenses[i] = expens
		}
	}
	return s
}

func getNth() int64 {
	var latest int64
	for _, share := range currentState.data {
		for _, expens := range share.Expenses {
			if expens.Nth > latest {
				latest = expens.Nth
			}
		}
	}
	return latest + 1
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
