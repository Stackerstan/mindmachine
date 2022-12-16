package patches

import (
	"io"
	"os"
	"strings"

	"mindmachine/mindmachine"
)

func srcRootDir() (d string) {
	home, err := os.UserHomeDir()
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
		return //IDE helper
	}
	return home + "/go/src/MindmachinePatches/"
}

func tempDir() string {
	err := os.MkdirAll(srcRootDir()+"tmp", 0777)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
		return "" //IDE helper
	}
	return srcRootDir() + "tmp/"
}

func isEmpty(location string) bool {
	f, err := os.Open(location)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			os.MkdirAll(location, 0777)
			return true
		}
		mindmachine.LogCLI(err.Error(), 1)
		return false
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true
	}
	mindmachine.LogCLI(err.Error(), 2)
	return false // Either not empty or some other filesystem error
}
