package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/stackerstan/go-nostr"
	"mindmachine/auxiliarium/patches"
	"mindmachine/auxiliarium/problems"
	"mindmachine/auxiliarium/protocol"
	"mindmachine/consensus/identity"
	"mindmachine/consensus/messagepack"
	"mindmachine/consensus/mindstate"
	"mindmachine/consensus/sequence"
	"mindmachine/consensus/shares"
	"mindmachine/messaging/blocks"
	"mindmachine/messaging/eventcatcher"
	"mindmachine/messaging/nostrelay"
	"mindmachine/mindmachine"
	"mindmachine/scumclass/eventbucket"
)

// cliListener is a cheap and nasty way to speed up development cycles. It listens for keypresses and executes commands.
func cliListener(interrupt chan struct{}) {
	fmt.Println("Press:\nq: to quit\ns: to print shares\ni: to print identity\nw: to print your current wallet\nSee cliListener.go for more")
	for {
		r, k, err := keyboard.GetSingleKey()
		if err != nil {
			panic(err)
		}
		str := string(r)
		switch str {
		default:
			if k == 13 {
				fmt.Println("\n-----------------------------------")
				break
			}
			if r == 0 {
				break
			}
			fmt.Println("Key " + str + " is not bound to any test procedures. See main.cliListener for more details.")
		case "q":
			//close(interrupt)
			mindmachine.Shutdown()
			messagepack.SealBlock(mindmachine.CurrentState().Processing.Height)
			go func() {
				mindmachine.LogCLI("User requested to terminate at block: "+fmt.Sprint(mindmachine.CurrentState().Processing.Height), 4)
				//If everything goes well, closing the interrupt channel should shutdown cleanly before terminating.
				//If something goes wrong,
				time.Sleep(time.Second * 10)
				println("Something didn't shutdown cleanly. In addition to whatever problem caused this, our " +
					"data is probably corrupt like our leaders.")
				os.Exit(0)
			}()
			return //if we do not return here, we cannot ctrl+c in case of errors during shutdown
		case "s":
			s := shares.MapOfCurrentState()
			fmt.Printf("%#v", s)
		case "w":
			fmt.Printf("\nWallet:\n%#v\nVotePower: %d\nCurrent Block:%v\n", mindmachine.MyWallet(), shares.VotePowerForAccount(mindmachine.MyWallet().Account), mindmachine.CurrentState().Processing)
		case "i":
			s := identity.GetMap()
			for account, accountIdentity := range s {
				fmt.Printf("%s:\n%#v\n", account, accountIdentity)
			}
		case "p":
			for s, repository := range patches.AllRepositories() {
				fmt.Println(s)
				m := repository.GetMapOfPatches()
				fmt.Printf("%#v\n", m)
				//if err != nil {
				//	mindmachine.LogCLI(err.Error(), 0)
				//for _, patch := range m {
				//	fmt.Printf("%#v\n", patch)
				//}
				//}
				if repository.Name == "mindmachine" {
					fmt.Println(repository.BuildTip())
				}
			}
		case "z":
			for _, repository := range patches.AllRepositories() {
				m := repository.GetMapOfPatches()
				var p []patches.Patch
				for _, patch := range m {
					p = append(p, patch)
				}
				sort.SliceStable(p, func(i, j int) bool {
					return p[i].Height < p[j].Height
				})
				for _, patch := range p {
					fmt.Printf("\nRepo: %s, UID: %s, Problem: %s, Creator: %s, Maintainer: %s, BasedOn: %s, Height: %d, CreatedAt: %d, Sequence: %d\n",
						patch.RepoName, patch.UID, "probs[patch.Problem].Title", patch.CreatedBy, patch.Maintainer, patch.BasedOn, patch.Height, patch.CreatedAt, patch.Sequence)
				}
			}
		case "o":
			var repo *patches.Repository
			for s, repository := range patches.AllRepositories() {
				fmt.Printf("\n%#v\n", repository)
				if s == "mindmachine" {
					repo = repository
					err := repo.BuildTip()
					if err != nil {
						mindmachine.LogCLI(err.Error(), 0)
					}
					fmt.Println(repo.StartWork("6c23235274802051c1a1f322ae4f02b6f4acfde64fb5c49105c15903c4e7eec7"))
				}
			}
		case "l":
			var repo *patches.Repository
			for s, repository := range patches.AllRepositories() {
				if s == "mindmachine" {
					repo = repository
				}
			}
			e, err := repo.CreateUnsignedPatchOffer("6c23235274802051c1a1f322ae4f02b6f4acfde64fb5c49105c15903c4e7eec7")
			if err != nil {
				mindmachine.LogCLI(err.Error(), 3)
			}
			for _, event := range e {
				if event.Kind == 641002 {
					event.Tags = nostr.Tags{[]string{"sequence", fmt.Sprintf("%d", sequence.GetSequence(mindmachine.MyWallet().Account)+1)}}
					event.Tags = append(event.Tags, []string{"height", fmt.Sprintf("%d", mindmachine.CurrentState().Processing.Height)})
				}
				event.Sign(mindmachine.MyWallet().PrivateKey)
				nostrelay.PublishEvent(event)
				time.Sleep(time.Millisecond * 200)
				//fmt.Printf("\n%s\n", event.ID)
				if event.Kind == 641002 {
					nostrelay.InjectEvent(event)
				}
			}
		case "m":
			var repo *patches.Repository
			for _, repository := range patches.AllRepositories() {
				if repository.Name == "mindmachine" {
					repo = repository
				}
			}
			if merge, err := repo.UnsignedMergePatchOffer("834d8b4ad8919869007cd77b5079cf25aaaf4f5fd64eeeddf2fc11b235bdefd2"); err == nil {
				merge.PubKey = mindmachine.IgnitionAccount
				if merge.Kind == 641004 {
					merge.Tags = nostr.Tags{[]string{"sequence", fmt.Sprintf("%d", sequence.GetSequence(mindmachine.MyWallet().Account)+1)}}
					merge.Tags = append(merge.Tags, []string{"height", fmt.Sprintf("%d", mindmachine.CurrentState().Processing.Height)})
				}
				merge.Sign("d836119ecf4700711e44acfd4f7878e1b40fa5c8a5df593dc043564ca710e35f")
				fmt.Printf("%#v", merge)
				nostrelay.InjectEvent(merge)
			} else {
				mindmachine.LogCLI(err.Error(), 2)
			}
		case "v":
			//increment by one block (careful!)
			go func() {
				behind := mindmachine.CurrentState().BitcoinTip.Height - mindmachine.CurrentState().Processing.Height
				current := mindmachine.CurrentState().Processing.Height
				if behind > 0 {
					next, err := blocks.FetchBlock(current + 1)
					if err != nil {
						mindmachine.LogCLI(err.Error(), 0)
					}
					blocks.InsertBlock(next)
				}
			}()
		case "b":
			//catch up to the current bitcoin tip (careful!)
			go func() {
				for mindmachine.CurrentState().BitcoinTip.Height-mindmachine.CurrentState().Processing.Height > 0 {
					next, err := blocks.FetchBlock(mindmachine.CurrentState().Processing.Height + 1)
					if err != nil {
						mindmachine.LogCLI(err.Error(), 0)
					}
					blocks.InsertBlock(next)
					time.Sleep(time.Second * 2)
				}
			}()
		case "P":
			for _, item := range protocol.GetProtocols() {
				fmt.Printf("\n%#v\n", item)
			}
			fmt.Println("----------FULL PROTOCOL FROM ROOT----------")
			var events []string
			fullprotocol := protocol.GetFullProtocol()
			for _, item := range fullprotocol {
				events = append(events, item.Text)
			}
			if fullEvents, ok := nostrelay.FetchEventPack(events); ok {
				for i, item := range fullprotocol {
					fmt.Printf("\n%s\nUID: %s\nNests: %s\n", fullEvents[i].Content, item.UID, item.Nests)
				}
			} else {
				mindmachine.LogCLI("could not fetch all events required", 3)
			}
		case "M":
			mp := messagepack.GetMessagePacks(760068)
			fmt.Println(mp)
		case "N":
			e := patches.GetLatestTip("mindmachine")
			bs, err := hex.DecodeString(e.Content)
			buf := bytes.Buffer{}
			//buf.WriteString(e.Content)
			buf.Write(bs)
			fileToWrite, err := os.OpenFile("./compress.tar.gzip", os.O_CREATE|os.O_RDWR, os.FileMode(777))
			if err != nil {
				mindmachine.LogCLI(err.Error(), 2)
			}
			_, err = io.Copy(fileToWrite, &buf)
			if err != nil {
				mindmachine.LogCLI(err.Error(), 2)
			}
		case "V":
			fmt.Printf("\n%#v\n", mindstate.GetLatestStates())
			for _, d := range mindstate.GetFullDB() {
				fmt.Printf("\n%#v\n", d)
			}
		case "K":
			m := mindmachine.GetAllKinds()
			unique := make(map[string]struct{})
			for _, s := range m {
				unique[s] = struct{}{}
			}
			var current string
			for s, _ := range unique {
				if s != current {
					current = s
					fmt.Println("\nMIND: ", s)
				}
				for i, s3 := range m {
					if s3 == s {
						fmt.Printf("Kind: %d\n", i)
					}
				}
			}
		case "e":
			p := problems.GetAllProblemsInOrder()
			for _, problem := range p {
				fmt.Printf("\n%#v\n", problem)
			}
		case "g":
			for _, state := range mindstate.GetLatestStates() {
				fmt.Printf("\n%#v\n", state)
			}
		case "O":
			fmt.Printf("\n%#v\n", mindstate.OpReturn())
		case "S":
			eventcatcher.Replay()
		case "d":
			//
			//go func() {
			//	var eventIDToSave string = ""
			//	pool := nostrelay.NewRelayPool()
			//	sub := pool.Sub(nostr.Filters{nostr.Filter{
			//		Kinds:   []int{640001},
			//		Authors: []string{mindmachine.MyWallet().Account},
			//	}})
			//	var events []nostr.Event
			//L:
			//	for {
			//		select {
			//		case e := <-sub.UniqueEvents:
			//			events = append(events, e)
			//		case <-time.After(time.Second * 5):
			//			break L
			//		}
			//	}
			//	n := nostr.Event{
			//		PubKey:    mindmachine.MyWallet().Account,
			//		CreatedAt: time.Now(),
			//		Kind:      5,
			//		Content:   "Rollback required to address bug",
			//	}
			//	tags := nostr.Tags{}
			//	for _, event := range events {
			//		if event.ID != eventIDToSave {
			//			tags = append(tags, []string{"e", event.ID})
			//		}
			//	}
			//	n.Tags = tags
			//	n.ID = n.GetID()
			//	n.Sign(mindmachine.MyWallet().PrivateKey)
			//	_, c, err := pool.PublishEvent(&n)
			//	if err != nil {
			//		mindmachine.LogCLI(err.Error(), 2)
			//	}
			//	for status := range c {
			//		fmt.Println(status)
			//	}
			//}()
		case "R":
			//go func() { nostrelay.RepublishEverything() }()
			go func() { nostrelay.PublishMissingEvents() }()
		case "E":
			event, ok := nostrelay.FetchLocalCachedEvent("9e333343184fe3e98b028782f7098cf596f1f46adf546541e7317d9a5f1d5d57")
			if ok {
				nostrelay.PublishEvent(event)
			}
		case "I":
			for _, kind := range eventbucket.GetNumberOfKinds() {
				fmt.Printf("Kind: %d Count: %d\n", kind.Kind, kind.Count)
			}
		}
	}
}
