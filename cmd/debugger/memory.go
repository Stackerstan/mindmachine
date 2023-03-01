package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/eiannone/keyboard"
)

func main() {
	go quitter()
	if len(os.Args[1:]) >= 1 {
		dirname := fmt.Sprintf("debug/mem/%d", time.Now().Unix())
		switch os.Args[1] {
		case "mem":
			for {
				if !logIt(dirname) {
					break
				}
				select {
				case <-time.After(time.Second * 30):
					if !logIt(dirname) {
						break
					}
				}
			}
		default:
			help()
		}
	} else {
		help()
	}

}

func quitter() {
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
			os.Exit(1)
		}
	}
}

func help() {
	fmt.Println()
	fmt.Println("STACKERSTAN DEBUGGER TOOL USAGE")
	fmt.Println()
	fmt.Println("This tool logs go profiling data from a running mindmachine process.")
	fmt.Println()
	fmt.Println("debugger <profile type> <mindmachine url (optional)> //types: mem")
	fmt.Println()
	fmt.Println()
}

func logIt(dirname string) (b bool) {
	var loc string = "localhost:8080"
	if len(os.Args[1:]) == 2 {
		loc = os.Args[2]
	}
	response, err := http.Get("http://" + loc + "/debug/pprof/heap")
	if err != nil {
		fmt.Println(err)
		return
	}
	buf := bytes.Buffer{}
	_, err = io.Copy(&buf, response.Body) //use package "io" and "os"
	if err != nil {
		fmt.Println(err)
		return
	}
	err = os.MkdirAll(dirname, 0777)
	if err != nil {
		fmt.Println(err)
		return
	}
	name := dirname + "/mem." + fmt.Sprintf("%d", time.Now().Unix()) + ".pprof"
	f, err := os.Create(name)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	n, err := f.Write(buf.Bytes())
	if err != nil {
		fmt.Println(err)
		return
	}
	err = f.Sync()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("\nwrote %d bytes to %s\n", n, name)
	return true
}
