package protocol

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/stackerstan/go-nostr"
	"mindmachine/mindmachine"
)

//15171031
func ignition(generate bool) []nostr.Event {
	createdAt := time.Unix(1667239800, 0)
	//if generate {
	//	createdAt = time.Now()
	//}
	pubkey := mindmachine.IgnitionAccount
	text := nostr.Event{
		PubKey:    pubkey,
		Kind:      1,
		Content:   "Stackerstan is an unstoppable civilisation built around Bitcoin",
		CreatedAt: createdAt,
		Tags:      nostr.Tags{},
	}
	text.ID = "a0284a96c828e10c869b9099cea3fa47de4ba5a22ef482afa00f6127734e9085"
	text.Sig = "53b71d69a053cec988f35e10ab588ffd2687098065e2e0d2fdeb3ed173112ebfa9dd00d321cb6f3062a3c7816a964136d1ccb189836a7b8038e583c14ac1167c"
	if generate {
		text.ID = text.GetID()
		text.Sign(mindmachine.MyWallet().PrivateKey)
	}
	tags := nostr.Tags{[]string{"height", fmt.Sprintf("%d", mindmachine.MakeOrGetConfig().GetInt64("ignitionHeight"))}, []string{"seq", fmt.Sprintf("%d", 3)}}
	content := Kind640600{
		Problem: "76b568caa18211152aba90826ded21e54096cec880a01fc2d9f8e7b344d6cfa6",
		Text:    "a0284a96c828e10c869b9099cea3fa47de4ba5a22ef482afa00f6127734e9085",
		Kind:    "goal",
		Parent:  "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b",
	}
	j, err := json.Marshal(content)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	protocolEvent := nostr.Event{
		PubKey:    pubkey,
		CreatedAt: createdAt,
		Kind:      640600,
		Tags:      tags,
		Content:   fmt.Sprintf("%s", j),
	}
	if generate {
		protocolEvent.ID = text.GetID()
		protocolEvent.Sign(mindmachine.MyWallet().PrivateKey)
	} else {
		protocolEvent.ID = "324552aac17d428498012359f90ea994bac00c1b979cb433ab1b06f61e99653b"
		protocolEvent.Sig = "e58bf131167c0ee07f550498ade4c348de6d26c6ea75b40ac99ac4087bdd0f6cc46e9cafd3018192194deef98b7f93a79f5a4efeb7d5330b0af10779985bc154"
	}
	if generate {
		fmt.Printf("\n%#v\n\n%#v\n\n%#v", text, protocolEvent, createdAt.Unix())
		//os.Exit(0)
	}
	if ok, err := protocolEvent.CheckSignature(); !ok {
		mindmachine.LogCLI(err.Error(), 0)
	}
	return []nostr.Event{text, protocolEvent}
}
