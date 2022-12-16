package main

import (
	"os"

	"github.com/spf13/viper"
	"mindmachine/mindmachine"
)

// initConfig sets up our Viper config object
func initConfig(config *viper.Viper) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	config.SetDefault("rootDir", homeDir+"/mindmachine/")
	config.SetConfigType("yaml")
	config.SetConfigFile(config.GetString("rootDir") + "config.yaml")
	err = config.ReadInConfig()
	if err != nil {
		mindmachine.LogCLI(err.Error(), 4)
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
	config.SetDefault("websocketAddr", "127.0.0.1:1031")
	config.SetDefault("fastSync", true)

	//we usually lean towards errors being fatal to cause less damage to state. If this is set to true, we lean towards staying alive instead.
	config.SetDefault("highly_reliable", false)
	config.SetDefault("forceBlocks", false)
	config.SetDefault("relays", []string{"wss://nostr.688.org"})
	config.SetDefault("optionalRelays", []string{"ws://127.0.0.1:8100"})
	// Create our working directory and config file if not exist
	initRootDir(config)
	mindmachine.Touch(config.GetString("rootDir") + "config.yaml")
	err = config.WriteConfig()
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
}

func initRootDir(conf *viper.Viper) {
	_, err := os.Stat(conf.GetString("rootDir"))
	if os.IsNotExist(err) {
		err = os.Mkdir(conf.GetString("rootDir"), 0755)
		if err != nil {
			mindmachine.LogCLI(err, 0)
		}
	}
}
