package shares

import (
	"fmt"
	"time"

	"github.com/fiatjaf/go-nostr"
	"mindmachine/mindmachine"
)

// GetIgnitionVPSS takes the current config and returns the ignition VPSS if
// it is needed (if we already have the initial share state, it is not needed).
// It returns true if the ignition VPSS is required to be processed.
func GetIgnitionVPSS() (e nostr.Event) {
	if !ignitionShareExists() {
		createIgnitionShares()
	}
	if ok, err := ignitionVPSS().CheckSignature(); !ok {
		mindmachine.LogCLI(err.Error(), 0)
	} else if ok {
		mindmachine.LogCLI("Dispatching our hard-coded ignition VPSS", 4)
	}
	return ignitionVPSS()
}

//15171031
func ignitionShareExists() bool {
	var ga mindmachine.Account
	if !mindmachine.MakeOrGetConfig().GetBool("devMode") {
		ga = mindmachine.IgnitionAccount
	}
	//if mindmachine.MakeOrGetConfig().GetBool("devMode") {
	//	ga = "1L9GQc3T3C1yZcNMEERt8XSBsgEzfGs2Rd"
	//}
	ignitionShares := current(ga)
	return ignitionShares.LeadTimeLockedShares > 0
}

func createIgnitionShares() mindmachine.HashSeq {
	shares := make(map[mindmachine.Account]Share)
	if mindmachine.MakeOrGetConfig().GetBool("devMode") {
		mindmachine.LogCLI("not implemented", 0)
	}
	//todo preflight
	if !mindmachine.MakeOrGetConfig().GetBool("devMode") {
		shares[mindmachine.IgnitionAccount] = Share{
			LeadTimeLockedShares: 1,
			LeadTime:             1,
			LastLtChange:         0,
		}
	}
	for account, share := range shares {
		update := share
		update.Sequence = 1
		update.LastLtChange = mindmachine.MakeOrGetConfig().GetInt64("ignitionHeight")
		shares[account] = update
	}
	for account, share := range shares {
		currentState.upsert(account, share)
	}
	fmt.Printf("\nCREATED IGNITION SHARES:\n%#v\n%#v\n", currentState.takeSnapshot(), currentState.data)
	return currentState.takeSnapshot()
}

func ignitionVPSS() (e nostr.Event) {
	content := mindmachine.Kind640000{
		Mind:      "shares",
		Hash:      "c55f081598d71f2d16dc0382ffa15ef455a2bdb77592b5b0203d46b80c3d51ca",
		Sequence:  1,
		ShareHash: "",
		Height:    mindmachine.MakeOrGetConfig().GetInt64("ignitionHeight"),
		EventID:   "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b",
	}
	j, err := json.Marshal(content)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	e = nostr.Event{
		ID:        "0921c6498a4093a344e1674375caf4a3d31447895b5272c7ca302090227f2b67",
		PubKey:    mindmachine.IgnitionAccount,
		CreatedAt: time.Unix(1667239800, 0), //createdat: 1667239800 height: 761151 ts: 1667239492 hash: 0000000000000000000040c44418efd4a6ffb03620266b5d802678031384e514
		Kind:      640000,
		Tags:      nostr.Tags{nostr.StringList{"height", "761151"}, nostr.StringList{"sequence", fmt.Sprintf("%d", 1)}},
		Content:   fmt.Sprintf("%s", j),
		Sig:       "213f27d24d073aedf2010f6cfb693aaa746b8234953a297d48293f844e2006897bd7ff958f6d88e543dca99d0e722e2979dc951227e34fb74d40ab7e7bd5b720",
	}
	//e.ID = e.GetID()
	//e.PubKey = mindmachine.MyWallet().Account
	//e.Sign(mindmachine.MyWallet().PrivateKey)
	//fmt.Printf("\n%#v\n%d\n", e, e.CreatedAt.Unix())
	if ok, _ := e.CheckSignature(); !ok {
		fmt.Printf("\n#\n")
		mindmachine.LogCLI("ignition vpss sig failed", 0)
	}
	return
}