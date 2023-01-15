package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/sasha-s/go-deadlock"
	"github.com/spf13/viper"
	"mindmachine/mindmachine"
	"mindmachine/scumclass/eventbucket"
)

func main() {
	deadlock.Opts.DisableLockOrderDetection = true
	deadlock.Opts.DeadlockTimeout = time.Millisecond * 60000
	beforeShutdown := beforeStart()
	g := eventbucket.EventBucket{}
	wordlist(&g)
	//fmt.Printf("\n%#v\n\n", g.SingleEvent("8f83827a02123fd59195425fcbeb6927a47a6322abf7af5ac54df30804763859").MentionMap)
	//fmt.Printf("\n%#v\n\n", g.SingleEvent("b40fb63a77fee76f5129fffe9c3f75b4aa64d153b5eccc7f2e5addf8f6775a6f").MentionMap)
	//fmt.Printf("\n%#v\n\n", g.SingleEvent("c9bcaf8e71dba904dbc901b2112f25cc9fef420afe171f39121ad7a3bc06af83").MentionMap)

	beforeShutdown()
}

func wordlist(g *eventbucket.EventBucket) {
	for _, cloud := range g.WordList(1) {
		fmt.Printf("\n\nEVENT TEXT: %s\n", cloud.EventContent)
		fmt.Println("--------------URLs---------------")
		for s, i := range cloud.URLs {
			fmt.Printf("%s: %d\n", s, i)
		}
		fmt.Println("--------------Keywords--------------")
		for s, i := range cloud.Keywords {
			fmt.Printf("%s: %d\n", s, i)
		}
		//fmt.Println("--------------WordMap---------------")
		//for s, i := range cloud.WordsMentioned {
		//	fmt.Printf("%s: %d\n", s, i)
		//}
		fmt.Println("-------------END----------------")
	}
}

func beforeStart() func() {
	terminator := make(chan struct{})
	wg := &sync.WaitGroup{}
	// Various aspect of this application require global and local settings. To keep things
	// clean and tidy we put these settings in a Viper configuration.
	conf := viper.New()

	// Now we initialise this configuration with basic settings that are required on startup.
	mindmachine.InitConfig(conf)

	// make the config accessible globally
	mindmachine.SetConfig(conf)
	eventbucket.StartDb(terminator, wg)
	//go eventcatcher.SubscribeToAllEvents(terminator)

	return func() {
		err := mindmachine.MakeOrGetConfig().WriteConfig()
		if err != nil {
			mindmachine.LogCLI(err.Error(), 3)
		}
		mindmachine.LogCLI("exiting", 3)
		close(terminator)
		wg.Wait()
		mindmachine.LogCLI("exited", 3)
	}
}
