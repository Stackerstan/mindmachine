package conductor

import (
	"sync"

	"mindmachine/auxiliarium/doki"
	"mindmachine/auxiliarium/patches"
	"mindmachine/auxiliarium/problems"
	"mindmachine/auxiliarium/protocol"
	"mindmachine/auxiliarium/samizdat"
	"mindmachine/consensus/identity"
	"mindmachine/consensus/messagepack"
	"mindmachine/consensus/mindstate"
	"mindmachine/consensus/sequence"
	"mindmachine/consensus/shares"
	"mindmachine/mindmachine"
)

var ready = make(chan struct{})

// Start starts the Conductor.
func Start(terminate chan struct{}, wg *sync.WaitGroup) {
	mindmachine.LogCLI("Starting the Conductor service (scum class Mind)", 4)
	go start(terminate, wg)
	<-ready
}
func start(terminate chan struct{}, wg *sync.WaitGroup) {
	// Add a waitgroup delta to the calling function so it knows we are doing something
	wg.Add(1)
	// We need a local waitgroup to wait for our databases to shut down when terminating the application.
	databaseWg := &sync.WaitGroup{}

	// We want to shut down databases only after everything else has shut down.
	terminateDatabases := make(chan struct{})

	// Start the shares database. Our consensus mechanism requires the shares module.
	shares.StartDb(terminateDatabases, databaseWg)

	// Now we start the Mind responsible for VotePower Signed State ("VPSS"). See the relevant Implemento
	// for more information about how VPSS is used to reach eventual consensus.
	mindstate.StartDb(terminateDatabases, databaseWg)

	//Process the ignition VPSS so that we have our starting point for consensus
	v := shares.GetIgnitionVPSS()
	ignitionHeight := mindmachine.MakeOrGetConfig().GetInt64("ignitionHeight")
	if ignitionHeight == mindmachine.CurrentState().Processing.Height {
		if _, ok := mindstate.HandleVPSS(mindmachine.ConvertToInternalEvent(&v)); !ok {
			mindmachine.LogCLI("this should not happen", 0)
		}
		mindstate.RegisterState(mindmachine.ConvertToInternalEvent(&v))
	}

	// Now we start all our other Minds
	sequence.StartDb(terminateDatabases, databaseWg)
	identity.StartDb(terminateDatabases, databaseWg)
	protocol.StartDb(terminateDatabases, databaseWg)
	problems.StartDb(terminateDatabases, databaseWg)
	patches.StartDb(terminateDatabases, databaseWg)
	doki.StartDb(terminateDatabases, databaseWg)
	samizdat.StartDb(terminateDatabases, databaseWg)

	//populate the bloom filter with all the events that we've seen so far so that we drop them if we see them again
	for _, s := range messagepack.GetMessagePacks(mindmachine.MakeOrGetConfig().GetInt64("ignitionHeight")) {
		bloom(s)
	}
	close(ready)
	// Now we sit here and wait for a signal from Main to terminate the application
	mindmachine.LogCLI("Conductor: I'm now accepting Events", 4)
	<-terminate
	mindmachine.LogCLI("Conductor: I received terminate signal, shutting down", 4)
	// Now that all the message handlers have shut down, we can close the databases
	close(terminateDatabases)
	databaseWg.Wait()
	mindmachine.LogCLI("Conductor: shutdown complete", 4)
	// Finally, tell our caller (probably Main) that we have completely shut down this Mind
	wg.Done()
}
