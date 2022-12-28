package mindmachine

import (
	"fmt"
	"strconv"

	"github.com/sasha-s/go-deadlock"
	"github.com/spf13/viper"
)

const IgnitionAccount = "543210b5f6c3071c3135d850449f8bf91efffb5ed1153e5fcbb2d95b79262b57"

var conf *viper.Viper

func MakeOrGetConfig() *viper.Viper {
	return conf
}

func SetConfig(config *viper.Viper) {
	conf = config
}

type State struct {
	Processing      BlockHeader
	BitcoinTip      BlockHeader //the current Bitcoin tip
	ProcessingEvent Event
	Shutdown        chan struct{}
}

var currentState = State{}
var stateMutex = &deadlock.Mutex{}

func Shutdown() {
	LogCLI("Calling Shutdown", 2)
	select {
	case <-currentState.Shutdown:
	default:
		close(currentState.Shutdown)
	}
}

func RegisterShutdownChan(shutdown chan struct{}) {
	currentState.Shutdown = shutdown
}

func CurrentState() (s State) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	if currentState.Processing.Height > 0 {
		s = currentState
	} else {
		LogCLI("current state requested before being set", 0)
	}
	return
}

func SetCurrentlyProcessing(b Event) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	if tags, ok := b.GetTags("block"); ok {
		bh := BlockHeader{
			Hash: tags[1],
		}
		time, err := strconv.ParseInt(tags[2], 10, 64)
		if err != nil {
			LogCLI(err, 0)
		}
		bh.Time = time
		height, err := strconv.ParseInt(tags[0], 10, 64)
		if err != nil {
			LogCLI(err, 0)
		}
		bh.Height = height
		currentState.Processing = bh
		currentState.ProcessingEvent = b
		return
	}
	fmt.Printf("%#v", b)
	LogCLI("Error parsing block", 0)
}

func SetBitcoinTip(bh BlockHeader) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	currentState.BitcoinTip = bh
}
