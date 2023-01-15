package mindmachine

import (
	"fmt"
	"log"
	"syscall"
)

func SetMaxOpenFiles() {
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
