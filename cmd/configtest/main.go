package main

import (
	"fmt"
	"log"
	"net/http"
	"syscall"

	"github.com/spf13/viper"
	"mindmachine/mindmachine"
)

func main() {
	response, err := http.Get("https://raw.githubusercontent.com/dskvr/nostr-watch/main/relays.yaml") //use package "net/http"

	if err != nil {
		fmt.Println(err)
		return
	}

	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		config := viper.New()
		config.SetConfigType("yaml")
		err = config.ReadConfig(response.Body)
		if err != nil {
			mindmachine.LogCLI(err.Error(), 2)
		}
		fmt.Println(config.GetStringSlice("relays"))
		maxOpenFiles()
		maxOpenFiles()
		//bodyBytes, err := io.ReadAll(response.Body)
		//if err != nil {
		//	log.Fatal(err)
		//}
		//bodyString := string(bodyBytes)
		//fmt.Println(bodyString)
	}

	// Copy data from the response to standard output
	//_, err1 := io.Copy(os.Stdout, response.Body) //use package "io" and "os"
	//if err != nil {
	//	fmt.Println(err1)
	//	return
	//}
	//err2 := ioutil.WriteFile("relays.yaml", response.Body, 0)
	//fmt.Println("Number of bytes copied to STDOUT:", n)
}

func maxOpenFiles() {
	var rLimit syscall.Rlimit

	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Println("Error Getting Rlimit ", err)
	}
	fmt.Println(rLimit.Cur)
	fmt.Println(rLimit.Max)
	fmt.Println()

	if rLimit.Cur < rLimit.Max {
		rLimit.Cur = rLimit.Max
		err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
		if err != nil {
			log.Println("Error Setting Rlimit ", err)
		}
	}
}
