package shares

import (
	"bytes"
	"fmt"

	"mindmachine/mindmachine"
)

func (e *Expense) hash() string {
	buf := &bytes.Buffer{}
	buf.WriteString(fmt.Sprint(e.UID, e.Problem, e.Solution, e.Amount, e.WitnessedAt, e.Approved))
	for blackballer := range e.Blackballers {
		buf.WriteString(blackballer)
	}
	for ratifier := range e.Ratifiers {
		buf.WriteString(ratifier)
	}
	return mindmachine.Sha256(buf.Bytes())
}

func (s *Share) AbsoluteVotePower() int64 {
	lt := s.LeadTime
	a := s.LeadTimeLockedShares
	return lt * a
}

func (s *Share) Permille() int64 {
	return mindmachine.Permille(s.AbsoluteVotePower(), totalVotePower())
}
