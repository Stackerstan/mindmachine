package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime/trace"
	"strings"
	"sync"
	"time"

	"github.com/sasha-s/go-deadlock"
	"github.com/spf13/viper"
	"mindmachine/messaging/blocks"
	"mindmachine/messaging/eventcatcher"
	"mindmachine/messaging/eventers"
	"mindmachine/messaging/nostrelay"

	//"github.com/pkg/profile"
	"mindmachine/mindmachine"
)

func main() {
	mindmachine.SetMaxOpenFiles()
	delve := false //problem: keep forgetting to turn this off after debug
	runtrace := false
	deadlock.Opts.DisableLockOrderDetection = true
	deadlock.Opts.DeadlockTimeout = time.Millisecond * 30000
	var f *os.File
	//p := profile.Start(profile.CPUProfile, profile.ProfilePath("."), profile.NoShutdownHook)
	if runtrace {
		// Delve does a much better job than GDB for Golang.
		// Profiling is useful for things like finding goroutines that have lost touch with reality.
		// https://github.com/DataDog/go-profiler-notes/blob/main/README.md
		var errt error
		f, errt = os.Create("trace.out")
		if errt != nil {
			panic(errt)
		}

		errt = trace.Start(f)
		if errt != nil {
			panic(errt)
		}
	}

	// Various aspect of this application require global and local settings. To keep things
	// clean and tidy we put these settings in a Viper configuration.
	conf := viper.New()

	// Now we initialise this configuration with basic settings that are required on startup.
	mindmachine.InitConfig(conf)
	// make the config accessible globally
	mindmachine.SetConfig(conf)
	if mindmachine.MakeOrGetConfig().GetBool("firstRun") {
		//Just giving everyone a chance to soak this up on the first run
		scanner := bufio.NewScanner(strings.NewReader(mindmachine.Banner()))
		for scanner.Scan() {
			time.Sleep(time.Millisecond * 127)
			fmt.Println(scanner.Text())
		}
		fmt.Println()
		time.Sleep(time.Second)
	} else {
		fmt.Printf("\n%s\n", mindmachine.Banner())
	}
	//507ms of silence in memory of those who have used branle
	time.Sleep(time.Millisecond * 507)

	// the terminator channel blocks until shutdown, anything requiring a clean shutdown should
	// wait on this channel and clean up when it stops blocking.
	terminator := make(chan struct{})

	// anything requiring a clean shutdown (databases etc) need to either directly or
	// by proxy add to this waitgroup and remove from this waitgroup when they
	// have cleanly shut down. Failure to do this will result in abandoned goroutines at sigterm.
	wg := &sync.WaitGroup{}

	// interrupt: see cliListener
	interrupt := make(chan struct{})

	//
	if delve {
		// If we've been waiting for a mutex lock for an Uncomfortable Period of Time (UPT),
		// we exit and dump all our goroutine stacks to the terminal. This is usually *very*
		// helpful *except* while debugging where breakpoints cause us to exceed UPT limits.
		deadlock.Opts.Disable = true
		//go debugTimeout(interrupt)
		//to kill it when debugging I just ps ax | grep debug; kill -9 <PID>, but in some situations
		//it makes more sense to uncomment the above and set an execution time limit.
	}
	if !delve {
		go cliListener(interrupt)
	}

	mindmachine.RegisterShutdownChan(interrupt)

	// Wait on the terminator channel. This blocks until the channel is closed by sending a shutdown signal.
	// This is blocking, so anything we need to start for normal operation MUST be started before this point.
	// Anything after this point will NOT run until the user or OS terminates the application.
	mindmachine.LogCLI("Waiting for terminate signal, press q to quit", 4)

	// Minds are something like Actors in the Actor model https://en.wikipedia.org/wiki/Actor_model
	// they have their own state ("Mind-state") and follow the SSP to update this state when they
	// receive a Nostr Event (https://github.com/nostr-protocol/nostr).
	go startMinds(terminator, wg, conf)

	<-interrupt
	//trace.Stop()
	mindmachine.MakeOrGetConfig().Set("firstRun", false)
	err := mindmachine.MakeOrGetConfig().WriteConfig()
	if err != nil {
		mindmachine.LogCLI(err.Error(), 3)
	}
	close(terminator)
	//p.Stop()
	if delve {
		//p.Stop()
	}
	if runtrace {
		trace.Stop()
		f.Close()
	}
	wg.Wait()
	os.Exit(0)
}

func debugTimeout(interrupt chan struct{}) {
	go func() {
		for {
			time.Sleep(time.Second * 5)
			behind := mindmachine.CurrentState().BitcoinTip.Height - mindmachine.CurrentState().Processing.Height
			current := mindmachine.CurrentState().Processing.Height
			if behind > 0 {
				next, err := blocks.FetchBlock(current + 1)
				if err != nil {
					mindmachine.LogCLI(err.Error(), 0)
				}
				blocks.InsertBlock(next)
			}
		}
	}()
	time.Sleep(time.Second * 1000)
	close(interrupt)
	time.Sleep(time.Second * 1)
	os.Exit(0)
}

// startMinds: Any Minds that need to be running during normal operation
// should be called directly or indirectly from here.
func startMinds(terminator chan struct{}, wg *sync.WaitGroup, conf *viper.Viper) {
	nostrelay.StartDb(terminator, wg)
	eventcatcher.Start(terminator, wg)
	eventers.Start()
}
