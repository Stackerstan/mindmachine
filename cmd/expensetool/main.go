package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"mindmachine/mindmachine"
)

func main() {
	if len(os.Args[1:]) == 1 {
		if len(os.Args[1]) == 40 {
			var repos = []string{"mindmachine", "interfarce", "go-nostr", "nostrkinds.org"}
			for _, repo := range repos {
				response, err := http.Get("https://github.com/Stackerstan/" + repo + "/commit/" + os.Args[1] + ".diff")
				if err == nil {
					buf := bytes.Buffer{}
					n, err1 := io.Copy(&buf, response.Body) //use package "io" and "os"
					if err != nil {
						fmt.Println(err1)
						return
					}
					if buf.String() != "Not Found" {
						hash := mindmachine.Sha256(buf.Bytes())
						fmt.Println("Number of bytes: ", n)
						fmt.Println("Hash: ", hash)
						response.Body.Close()
						return
					}
				}
			}
			fmt.Println("ERROR: could not find any commit with hash: " + os.Args[1])
			return
		}
	}
	fmt.Println()
	fmt.Println("STACKERSTAN EXPENSE TOOL USAGE")
	fmt.Println()
	fmt.Println("This tool fetches a commit from github and gives you a hash of the corresponding patch. It only works on Stackerstan repositories.")
	fmt.Println()
	fmt.Println("Go to a Stackerstan repository that you have contributed to, find your *merged* pull request. Copy the commit hash (40 characters in length), and then:")
	fmt.Println()
	fmt.Println("expensetool <git commit hash>")
	fmt.Println()
	fmt.Println()
}
