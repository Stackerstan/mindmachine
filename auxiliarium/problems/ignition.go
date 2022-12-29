package problems

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/stackerstan/go-nostr"
	"mindmachine/mindmachine"
)

//15171031
func ignition(generate bool) []nostr.Event {
	createdAt := time.Unix(1667239800, 0)
	pubkey := mindmachine.IgnitionAccount
	title := nostr.Event{
		PubKey:    pubkey,
		Kind:      1,
		Content:   "Humanity is not living up to its full potential",
		CreatedAt: createdAt,
		Tags:      nostr.Tags{},
	}
	description := nostr.Event{
		PubKey:    pubkey,
		Kind:      1,
		Content:   "All problems in the problem tracker must be nested under another problem, creating a tree structure.\n\nThis initial problem is intended to set the scope of all possible problems that MAY become applicable to Stackerstan - in the broadest possible sense, this is the ultimate problem, and the reason for creating this project.",
		CreatedAt: createdAt,
		Tags:      nostr.Tags{},
	}
	if generate {
		title.ID = title.GetID()
		title.Sign(mindmachine.MyWallet().PrivateKey)
		description.ID = title.GetID()
		description.Sign(mindmachine.MyWallet().PrivateKey)
	} else {
		//15171031
		title.ID = "8d1732ceb07619634e5b863003504a52c28f5593c10d9e650e4d320a730bfefe"
		title.Sig = "0ace2ed14f85558d330c53c1443c8514b06910d32258216eac8634b46dc4493f5b41c12a3766acb5f43660961f902b030d3a28072208a5c22c0b325eefbcd0af"
		description.ID = "b22ddf824cc4fc0824c65393488bd47a3a7ef841ae2f04d91853ee80f08cda66"
		description.Sig = "9432b62321ed8b9d5138214b94c1f4c6ef9a0321a3a2d22bda518d364cc509a7a78771eea9f507df7af634239e2b0d980b49eb81e80ea9c7c6e3c5e51c536971"
	}
	tags := nostr.Tags{[]string{"height", fmt.Sprintf("%d", mindmachine.MakeOrGetConfig().GetInt64("ignitionHeight"))}, []string{"seq", fmt.Sprintf("%d", 2)}}
	content := Kind640800{
		Title:       title.ID,
		Description: description.ID,
		Parent:      "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b",
	}
	j, err := json.Marshal(content)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	problemEvent := nostr.Event{
		PubKey:    pubkey,
		CreatedAt: createdAt,
		Kind:      640800,
		Tags:      tags,
		Content:   fmt.Sprintf("%s", j),
	}
	if generate {
		problemEvent.ID = title.GetID()
		problemEvent.Sign(mindmachine.MyWallet().PrivateKey)
	} else {
		problemEvent.ID = "3d666acf9483baed64213e79aad72f1db808a903ccd5e2c0fb48388fe6eb89e6"
		problemEvent.Sig = "255334c6d1d5b228c2e27d2b6d325d71c53b04b68dc5831e89059254e532081bd2153878bc306a0146049684a436e24bed8dae6717c9f823334e77cafb1f95c6"
	}
	if generate {
		fmt.Printf("\n%#v\n\n%#v\n\n%#v\n\n%d", title, description, problemEvent, createdAt.Unix())
		os.Exit(0)
	}
	return []nostr.Event{title, description, problemEvent}
}
