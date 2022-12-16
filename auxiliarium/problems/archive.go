package problems

import (
	"encoding/json"
	"sort"

	"mindmachine/database"
	"mindmachine/mindmachine"
)

// takeSnapshot calculates a hash (and gets the total sequence) at the current state. It also stores the state in the
//database, indexed by hash of the state. It returns the hash and sequence.
func (s *db) takeSnapshot() mindmachine.HashSeq {
	hs := hashSeq(s.data)
	b, err := json.MarshalIndent(s.data, "", " ")
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	database.Write("problems", hs.Hash, b)
	return hs
}

func hashSeq(m map[mindmachine.S256Hash]Problem) (hs mindmachine.HashSeq) {
	hs.Mind = "problems"
	var uids []mindmachine.S256Hash
	for uid := range m {
		uids = append(uids, uid)
	}
	sort.Slice(uids, func(i, j int) bool {
		return uids[i] > uids[j]
	})
	for _, uid := range uids {
		if item, ok := m[uid]; !ok {
			mindmachine.LogCLI("this should not happen", 0)
		} else {
			hs.Sequence = hs.Sequence + item.Sequence
			if err := hs.AppendData(item.UID); err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			if err := hs.AppendData(item.WitnessedAt); err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			if err := hs.AppendData(item.LastUpdate); err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			if err := hs.AppendData(item.Sequence); err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			if err := hs.AppendData(item.CreatedBy); err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			if err := hs.AppendData(item.Parent); err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			if err := hs.AppendData(item.ClaimedBy); err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			if err := hs.AppendData(item.Closed); err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			if err := hs.AppendData(item.Description); err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			if err := hs.AppendData(item.Title); err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			if err := hs.AppendData(item.ClaimedAt); err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			if err := hs.AppendData(item.Curator); err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			}
			for _, nest := range item.Children {
				if err := hs.AppendData(nest); err != nil {
					mindmachine.LogCLI(err.Error(), 1)
				}
			}
		}
	}
	hs.S256()
	hs.CreatedAt = mindmachine.CurrentState().Processing.Height
	return
}
