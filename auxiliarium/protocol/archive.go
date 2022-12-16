package protocol

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
	database.Write("protocol", hs.Hash, b)
	return hs
}

func hashSeq(m map[mindmachine.S256Hash]Item) (hs mindmachine.HashSeq) {
	hs.Mind = "protocol"
	var uids []mindmachine.S256Hash
	for uid := range m {
		uids = append(uids, uid)
	}
	sort.Slice(uids, func(i, j int) bool {
		return uids[i] > uids[j]
	})
	var toHash []interface{}
	for _, uid := range uids {
		toHash = append(toHash, uid)
		if item, ok := m[uid]; !ok {
			mindmachine.LogCLI("this should not happen", 0)
		} else {
			hs.Sequence = hs.Sequence + item.Sequence
			toHash = append(toHash,
				item.WitnessedAt,
				item.LastUpdate,
				item.Sequence,
				item.CreatedBy,
				item.SupersededBy,
				item.ApprovedAt,
				item.Supersedes,
				item.Text,
				item.Kind,
				item.Problem)
			for _, nest := range item.Nests {
				toHash = append(toHash, nest)
			}
			var voters []mindmachine.Account
			for account, _ := range item.Ratifiers {
				voters = append(voters, account)
			}
			for account, _ := range item.Blackballers {
				voters = append(voters, account)
			}
			sort.Slice(voters, func(i, j int) bool {
				return voters[i] > voters[j]
			})
			for _, voter := range voters {
				toHash = append(toHash, voter)
			}
		}
	}
	for _, d := range toHash {
		if err := hs.AppendData(d); err != nil {
			mindmachine.LogCLI(err, 0)
		}
	}
	hs.S256()
	hs.CreatedAt = mindmachine.CurrentState().Processing.Height
	return
}
