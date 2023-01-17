package mindmachine

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/viper"
)

// InitConfig sets up our Viper config object
func InitConfig(config *viper.Viper) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		LogCLI(err.Error(), 0)
	}
	config.SetDefault("rootDir", homeDir+"/mindmachine/")
	config.SetConfigType("yaml")
	config.SetConfigFile(config.GetString("rootDir") + "config.yaml")
	err = config.ReadInConfig()
	if err != nil {
		LogCLI(err.Error(), 4)
	}
	config.SetDefault("firstRun", true)
	config.SetDefault("flatFileDir", "data/")
	config.SetDefault("blockServer", "https://blockchain.info")
	config.SetDefault("logLevel", 4)
	config.SetDefault("logActors", true)
	config.SetDefault("devMode", false)
	if config.GetBool("devMode") {
		config.SetDefault("ignitionHeight", int64(0))
	}
	if !config.GetBool("devMode") {
		config.SetDefault("ignitionHeight", int64(761151))
	}
	config.SetDefault("websocketAddr", "0.0.0.0:1031")
	config.SetDefault("fastSync", true)

	//we usually lean towards errors being fatal to cause less damage to state. If this is set to true, we lean towards staying alive instead.
	config.SetDefault("highly_reliable", false)
	config.SetDefault("forceBlocks", false)
	config.SetDefault("relaysMust", []string{"wss://nostr.688.org"})
	if optionalRelays, ok := getOptionalRelays(); ok {
		fmt.Println(44)
		fmt.Println(len(optionalRelays))
		config.SetDefault("relaysOptional", optionalRelays)
		config.Set("relaysOptional", optionalRelays)
	}
	// Create our working directory and config file if not exist
	initRootDir(config)
	Touch(config.GetString("rootDir") + "config.yaml")
	err = config.WriteConfig()
	if err != nil {
		LogCLI(err.Error(), 0)
	}
}

func initRootDir(conf *viper.Viper) {
	_, err := os.Stat(conf.GetString("rootDir"))
	if os.IsNotExist(err) {
		err = os.Mkdir(conf.GetString("rootDir"), 0755)
		if err != nil {
			LogCLI(err, 0)
		}
	}
}

func getOptionalRelays() ([]string, bool) {
	LogCLI("fetching optional relays from nostr-watch", 4)
	response, err := http.Get("https://raw.githubusercontent.com/dskvr/nostr-watch/main/relays.yaml")

	if err == nil {
		defer response.Body.Close()
		if response.StatusCode == http.StatusOK {
			config := viper.New()
			config.SetConfigType("yaml")
			err = config.ReadConfig(response.Body)
			if err == nil {
				relays := config.GetStringSlice("relays")
				if len(relays) > 0 {
					return relays, true
				}
			}
		}
	} else {
		LogCLI(err.Error(), 4)
	}

	return []string{}, false
}
