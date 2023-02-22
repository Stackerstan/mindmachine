// Package eventers lets us compose State from any Mind(s) into Events to be consumed by the interfarce.
//all Events produce by the eventer are signed by our local wallet
//events produced by this package MUST NOT be sent to relays (except maybe to fiatjaf's relays as punishment for branle)
package eventers

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/stackerstan/go-nostr"
	"mindmachine/auxiliarium/doki"
	"mindmachine/auxiliarium/patches"
	"mindmachine/auxiliarium/problems"
	"mindmachine/auxiliarium/protocol"
	"mindmachine/auxiliarium/samizdat"
	"mindmachine/consensus/identity"
	"mindmachine/consensus/mindstate"
	"mindmachine/consensus/sequence"
	"mindmachine/consensus/shares"
	"mindmachine/messaging/nostrelay"
	"mindmachine/mindmachine"
	"mindmachine/scumclass/eventbucket"
	"mindmachine/scumclass/nostrkinds"
)

func Start() {
	mindmachine.LogCLI("Starting the Event Producer. You should now be able to connect a frontend to this Mindmachine instance.", 4)
	go startResponding()
}

func startResponding() {
	subs := nostrelay.SubscribeToRequests("eventer")
	all := nostrelay.SubscribeToRequests("all")
	for {
		select {
		case newSub := <-subs:
			go handleSubscription(newSub)
		case newSub := <-all:
			//fmt.Printf("\n%#v\n", newSub.Filters)
			go handleSubscription(newSub)
		}
	}
}

func handleSubscription(sub nostrelay.Subscription) {
	for _, filter := range sub.Filters {
		for s, list := range filter.Tags {
			if s == "eventer" {
				switch list[0] {
				case "protocol":
					for _, event := range fullProtocol() {
						sub.Events <- event
					}
					close(sub.Terminate)
				case "problems":
					for _, event := range allProblems() {
						sub.Events <- event
					}
					close(sub.Terminate)
				case "mindmachineTip":
					p := patches.GetLatestTip("mindmachine")
					sub.Events <- p
					close(sub.Terminate)
				case "identity":
					for _, event := range allIdentities() {
						sub.Events <- event
						//fmt.Println(event.ID)
					}
					close(sub.Terminate)
				case "doki":
					for _, event := range allDoki() {
						//fmt.Println(event.ID)
						sub.Events <- event
					}
					close(sub.Terminate)
				case "samizdat":
					for _, event := range allSamizdat() {
						sub.Events <- event
					}
					close(sub.Terminate)
				case "patches":
					for _, event := range getAllPatches() {
						sub.Events <- event
					}
					close(sub.Terminate)
				case "currentstate":
					//todo keep this updated by sending new event every block. We could do this by subscribing to mindmachine.CurrentState
					sub.Events <- currentState()
					close(sub.Terminate)
				case "eventbucket":
					for _, event := range getAllEventBucketKinds() {
						sub.Events <- event
					}
					close(sub.Terminate)
				case "shares":
					for _, event := range getAllShares() {
						sub.Events <- event
					}
					close(sub.Terminate)
				case "kinds":
					for _, event := range getAllEventKinds() {
						sub.Events <- event
					}
					close(sub.Terminate)
				default:
					close(sub.Terminate)
				}
				return
			}
		}
		if len(filter.Authors) > 0 {
			if len(filter.Kinds) > 0 {
				for _, kind := range filter.Kinds {
					if kind == 0 {
						var events []nostr.Event
						for _, author := range filter.Authors {
							if e, ok := eventbucket.GetKind0(author); ok {
								events = append(events, e)
							}
						}
						for _, event := range events {
							sub.Events <- event
						}
					}
				}
			}
		}
	}
}

func getAllShares() (e []nostr.Event) {
	//need shares on the frontend because we need the sequence number to create an expense
	all := make(map[string]shares.Share)
	for account, share := range shares.MapOfCurrentState() {
		all[account] = share
	}
	j, err := json.Marshal(all)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 1)
	} else {
		e = append(e, nostr.Event{
			PubKey:    mindmachine.MyWallet().Account,
			CreatedAt: time.Now(),
			Kind:      640299,
			Tags:      nil,
			Content:   fmt.Sprintf("%s", j),
		})
	}
	return
}

func getAllEventKinds() (e []nostr.Event) {
	kinds := nostrkinds.GetAll()
	for _, kind := range kinds {
		j, err := json.Marshal(kind)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 1)
		} else {
			ev := nostr.Event{
				PubKey:    mindmachine.MyWallet().Account,
				CreatedAt: time.Now(),
				Kind:      641899,
				Tags:      nil,
				Content:   fmt.Sprintf("%s", j),
			}
			ev.ID = ev.GetID()
			ev.Sign(mindmachine.MyWallet().PrivateKey)
			e = append(e, ev)
		}
	}
	event := nostr.Event{
		PubKey:    mindmachine.MyWallet().Account,
		CreatedAt: time.Now(),
		Kind:      641851,
		Content:   "",
	}
	event.ID = event.GetID()
	event.Sign(mindmachine.MyWallet().PrivateKey)
	e = append(e, event)
	return e
}

func getAllEventBucketKinds() (e []nostr.Event) {
	kinds := eventbucket.GetNumberOfKinds()
	for _, kind := range kinds {
		j, err := json.Marshal(kind)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 1)
		} else {
			ev := nostr.Event{
				PubKey:    mindmachine.MyWallet().Account,
				CreatedAt: time.Now(),
				Kind:      641699,
				Tags:      nil,
				Content:   fmt.Sprintf("%s", j),
			}
			ev.ID = ev.GetID()
			ev.Sign(mindmachine.MyWallet().PrivateKey)
			e = append(e, ev)
		}
	}
	event := nostr.Event{
		PubKey:    mindmachine.MyWallet().Account,
		CreatedAt: time.Now(),
		Kind:      641651,
		Content:   "",
	}
	event.ID = event.GetID()
	event.Sign(mindmachine.MyWallet().PrivateKey)
	e = append(e, event)
	return e
}

type Patch struct {
	RepoName   string
	CreatedBy  mindmachine.Account
	Diff       string
	UID        mindmachine.S256Hash // hash of Diff
	Problem    mindmachine.S256Hash // the hash of the problem from the problem tracker
	Maintainer mindmachine.Account  // the account that merged this patch
	BasedOn    mindmachine.S256Hash // the patch that this patch is based on
	Height     int64                // height of this patch in the patch chain
	CreatedAt  int64                // BTC height when this patch was created
	Conflicts  bool
	Sequence   int64
}

func getAllPatches() (e []nostr.Event) {
	var repo *patches.Repository
	var foundit = false
	for s, repository := range patches.AllRepositories() {
		if s == "mindmachine" {
			repo = repository
			foundit = true
		}
	}
	var patchList []Patch
	if foundit {
		for _, patch := range repo.GetMapOfPatches() {
			b, err := hex.DecodeString(fmt.Sprintf("%x", patch.Diff))
			if err != nil {
				mindmachine.LogCLI(err.Error(), 2)
			}
			patchList = append(patchList, Patch{
				RepoName:   patch.RepoName,
				CreatedBy:  patch.CreatedBy,
				UID:        patch.UID,
				Diff:       fmt.Sprintf("%s", b),
				Problem:    patch.Problem,
				Maintainer: patch.Maintainer,
				BasedOn:    patch.BasedOn,
				Height:     patch.Height,
				CreatedAt:  patch.CreatedAt,
				Conflicts:  patch.Conflicts,
				Sequence:   patch.Sequence,
			})

		}
		sort.Slice(patchList, func(i, j int) bool {
			return patchList[i].CreatedAt > patchList[j].CreatedAt
		})

		for _, patch := range patchList {
			j, err := json.Marshal(patch)
			if err != nil {
				mindmachine.LogCLI(err.Error(), 1)
			} else {
				ev := nostr.Event{
					PubKey:    mindmachine.MyWallet().Account,
					CreatedAt: time.Now(),
					Kind:      641097,
					Tags:      nil,
					Content:   fmt.Sprintf("%s", j),
				}
				ev.ID = ev.GetID()
				ev.Sign(mindmachine.MyWallet().PrivateKey)
				e = append(e, ev)
			}
		}
	}

	event := nostr.Event{
		PubKey:    mindmachine.MyWallet().Account,
		CreatedAt: time.Now(),
		Kind:      641051,
		Content:   "",
	}
	event.ID = event.GetID()
	event.Sign(mindmachine.MyWallet().PrivateKey)
	e = append(e, event)
	return e
}

type state struct {
	Height       int64
	Shares       int64
	Votepower    int64
	Participants int64
	Maintainers  int64
	Shareholders int64
	OpReturn     string
}

var current state

func currentState() nostr.Event {
	latest := mindmachine.CurrentState().Processing.Height
	if current.Height != latest {
		allshares := shares.MapOfCurrentState()
		var totalShares int64
		var totalVotepower int64
		for _, share := range allshares {
			totalShares += share.LeadTimeUnlockedShares
			totalShares += share.LeadTimeLockedShares
			totalVotepower += share.LeadTimeLockedShares * share.LeadTime
		}
		allParticipants := identity.GetMap()
		var p int64
		var m int64
		for _, i := range allParticipants {
			if len(i.UniqueSovereignBy) > 0 {
				p++
			}
			if len(i.MaintainerBy) > 0 {
				m++
			}
		}

		opreturnset := mindstate.OpReturn()
		newState := state{
			Height:       latest,
			Shares:       totalShares,
			Votepower:    totalVotepower,
			Participants: p,
			Maintainers:  m,
			Shareholders: int64(len(allshares)),
			OpReturn:     opreturnset.OpReturn,
		}
		current = newState
	}
	j, err := json.Marshal(current)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	e := nostr.Event{
		PubKey:    mindmachine.MyWallet().Account,
		CreatedAt: time.Now(),
		Kind:      649999,
		Tags:      nil,
		Content:   fmt.Sprintf("%s", j),
	}
	e.ID = e.GetID()
	e.Sign(mindmachine.MyWallet().PrivateKey)
	return e
}

func allSamizdat() (e []nostr.Event) {
	list := samizdat.AllSamizdat()
	events, _ := nostrelay.FetchEventPack(list)
	for _, event := range events {
		e = append(e, event.Nostr())
	}
	event := nostr.Event{
		PubKey:    mindmachine.MyWallet().Account,
		CreatedAt: time.Now(),
		Kind:      641497,
		Content:   "",
	}
	event.ID = event.GetID()
	event.Sign(mindmachine.MyWallet().PrivateKey)
	e = append(e, event)
	return
}

func allIdentities() (e []nostr.Event) {
	idents := identity.GetMap()
	var identSlice []identity.Identity
	identSlice = append(identSlice, idents[mindmachine.IgnitionAccount])
	for _, i := range idents {
		if i.Order > 0 {
			identSlice = append(identSlice, i)
		}
	}
	sort.Slice(identSlice, func(i, j int) bool {
		return identSlice[i].Order < identSlice[j].Order
	})
	for _, i := range idents {
		if i.Order == 0 && i.Account != mindmachine.IgnitionAccount {
			identSlice = append(identSlice, i)
		}
	}

	for _, i := range identSlice {
		event := nostr.Event{
			PubKey:    mindmachine.MyWallet().Account,
			CreatedAt: time.Now(),
			Kind:      640499,
		}
		content := Kind640499{
			Account:        i.Account,
			Name:           i.Name,
			About:          i.About,
			Sequence:       i.Sequence,
			UshBy:          i.UniqueSovereignBy,
			MaintainerBy:   i.MaintainerBy,
			GlobalSequence: sequence.GetSequence(i.Account),
			Order:          i.Order,
		}
		s, err := json.Marshal(content)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 1)
			return []nostr.Event{}
		}
		event.Content = fmt.Sprintf("%s", s)
		event.ID = event.GetID()
		event.Sign(mindmachine.MyWallet().PrivateKey)
		e = append(e, event)
	}
	return
}

type Kind640499 struct {
	Account        string
	Name           string
	About          string
	Sequence       int64
	UshBy          string
	MaintainerBy   string
	GlobalSequence int64
	Order          int64
}

func allDoki() (e []nostr.Event) {
	docs := doki.GetAll()
	for _, document := range docs {
		j, err := json.Marshal(document)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 1)
			return
		}
		event := nostr.Event{
			PubKey:    mindmachine.MyWallet().Account,
			CreatedAt: time.Now(),
			Kind:      641299,
			Content:   fmt.Sprintf("%s", j),
		}
		event.ID = event.GetID()
		event.Sign(mindmachine.MyWallet().PrivateKey)
		e = append(e, event)
	}
	event := nostr.Event{
		PubKey:    mindmachine.MyWallet().Account,
		CreatedAt: time.Now(),
		Kind:      641297,
		Content:   "",
	}
	event.ID = event.GetID()
	event.Sign(mindmachine.MyWallet().PrivateKey)
	e = append(e, event)
	return
}

func allProblems() (e []nostr.Event) {
	for _, item := range problems.GetAllProblemsInOrder() {
		event := nostr.Event{
			PubKey:    mindmachine.MyWallet().Account,
			CreatedAt: time.Now(),
			Kind:      640899,
			//Tags:      makeProblemTags(item),
		}
		var content string
		if evts, ok := nostrelay.FetchEventPack([]string{item.Title, item.Description}); ok {
			//content = evts[0].Content + "\n\n"
			content += evts[1].Content
			if len(evts[0].Content) > 0 {
				event.Tags = makeProblemTags(item, evts[0].Content)
				if len(evts[1].Content) > 0 {
					event.Content = content
					event.Sign(mindmachine.MyWallet().PrivateKey)
					e = append(e, event)
				}
			}
		}
	}
	event := nostr.Event{
		PubKey:    mindmachine.MyWallet().Account,
		CreatedAt: time.Now(),
		Kind:      640897,
		Content:   "",
	}
	event.ID = event.GetID()
	event.Sign(mindmachine.MyWallet().PrivateKey)
	e = append(e, event)
	return
}

func fullProtocol() (e []nostr.Event) {
	var events []string
	fullprotocol := protocol.GetFullProtocol()
	for _, item := range fullprotocol {
		events = append(events, item.Text)
	}
	if fullEvents, ok := nostrelay.FetchEventPack(events); ok {
		for i, item := range fullprotocol {
			protocolEvent := nostr.Event{
				PubKey:    mindmachine.MyWallet().Account,
				CreatedAt: time.Now(),
				Kind:      640699,
				Tags:      makeProtocolTags(item),
				Content:   fullEvents[i].Content,
			}
			protocolEvent.Sign(mindmachine.MyWallet().PrivateKey)
			e = append(e, protocolEvent)
		}
		event := nostr.Event{
			PubKey:    mindmachine.MyWallet().Account,
			CreatedAt: time.Now(),
			Kind:      640697,
			Content:   "",
		}
		event.ID = event.GetID()
		event.Sign(mindmachine.MyWallet().PrivateKey)
		e = append(e, event)
	} else {
		mindmachine.LogCLI("could not fetch all events required", 3)
	}
	return
}

func makeProblemTags(item problems.Problem, title string) (t nostr.Tags) {
	if len(item.Children) > 0 {
		var children = []string{"children"}
		for _, child := range item.Children {
			children = append(children, child)
		}
	}
	if len(item.Parent) > 0 {
		t = append(t, []string{"parent", item.Parent})
	}
	t = append(t, []string{"height", fmt.Sprintf("%d", item.WitnessedAt)})
	t = append(t, []string{"mindmachineUID", item.UID})
	t = append(t, []string{"mind", "problems"})
	t = append(t, []string{"sequence", fmt.Sprintf("%d", item.Sequence)})
	t = append(t, []string{"repo", ""})
	t = append(t, []string{"title", title})
	t = append(t, []string{"claimed_by", item.ClaimedBy})

	return
}

func makeProtocolTags(item protocol.Item) (t nostr.Tags) {
	switch item.Kind {
	case protocol.Definition:
		t = append(t, []string{"kind", "Definition"})
	case protocol.Goal:
		t = append(t, []string{"kind", "Goal"})
	case protocol.Rule:
		t = append(t, []string{"kind", "Rule"})
	case protocol.Invariant:
		t = append(t, []string{"kind", "Invariant"})
	case protocol.Protocol:
		t = append(t, []string{"kind", "Protocol"})
	}
	var nests = []string{"nests"}
	for _, nest := range item.Nests {
		nests = append(nests, nest)
	}
	if len(nests) > 1 {
		t = append(t, nests)
	}
	t = append(t, []string{"mindmachineUID", item.UID})
	t = append(t, []string{"mind", "protocol"})
	return
}
