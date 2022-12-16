package mindstate

import (
	"bytes"
	"encoding/hex"
	"sort"

	"mindmachine/consensus/messagepack"
	"mindmachine/mindmachine"
)

type OpReturnData struct {
	OpReturn string
	Events   []string
	Minds    []MindState
}

func OpReturn() (opr OpReturnData) {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()

	eventList := messagepack.GetMessagePacks(mindmachine.MakeOrGetConfig().GetInt64("ignitionHeight"))
	buf := bytes.Buffer{}
	for _, s := range eventList {
		buf.WriteString(s)
	}
	bsEventListHash, err := hex.DecodeString(mindmachine.Sha256(buf.Bytes()))
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	latest := getLatestStatus(true)
	var toMerkle [][]byte
	var sortedMindNames []string
	for s, _ := range latest {
		sortedMindNames = append(sortedMindNames, s)
	}
	sort.SliceStable(sortedMindNames, func(i, j int) bool {
		return sortedMindNames[i] < sortedMindNames[j]
	})
	for _, name := range sortedMindNames {
		bs, err := hex.DecodeString(latest[name].State)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 0)
		}
		toMerkle = append(toMerkle, bs)
		opr.Minds = append(opr.Minds, latest[name])
	}
	toMerkle = append(toMerkle, bsEventListHash)
	mklRoot := mindmachine.Merkle(toMerkle)[0]
	opr.OpReturn = hex.EncodeToString(mklRoot)
	opr.Events = eventList
	return
}
