package nostrelay

import (
	"time"

	"github.com/fiatjaf/go-nostr"
	"mindmachine/mindmachine"
)

//15171031
func GetIgnitionBlock() mindmachine.Event {
	b := nostr.Event{
		PubKey:    mindmachine.MyWallet().Account,
		CreatedAt: time.Now(),
		Kind:      125,
		Tags: nostr.Tags{
			nostr.StringList{"block", "761151", "0000000000000000000040c44418efd4a6ffb03620266b5d802678031384e514", "1667239492"},
			nostr.StringList{"mind", "blocks"},
		},
	}
	b.Sign(mindmachine.MyWallet().PrivateKey)
	block := mindmachine.ConvertToInternalEvent(&b)
	return block
}