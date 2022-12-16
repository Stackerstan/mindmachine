package patches

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fiatjaf/go-nostr"
	"mindmachine/mindmachine"
)

func GetLatestTip(repo string) (n nostr.Event) {
	for _, repository := range currentState.data {
		if repository.Name == repo {
			err := repository.BuildTip()
			if err != nil {
				mindmachine.LogCLI(err.Error(), 1)
				return
			}
			path := repository.rootDir() + "TIP"
			// tar + gzip
			var buf bytes.Buffer
			err = compressTip(path, &buf, repo)
			if err != nil {
				mindmachine.LogCLI(err.Error(), 1)
				return
			}
			n.PubKey = mindmachine.MyWallet().Account
			n.CreatedAt = time.Now()
			n.Kind = 641099
			n.Content = fmt.Sprintf("%x", buf.Bytes())
			n.Sign(mindmachine.MyWallet().PrivateKey)
			return
		}
	}
	return
}

func compressTip(src string, buf io.Writer, repo string) error {
	// tar > gzip > buf
	zr := gzip.NewWriter(buf)
	tw := tar.NewWriter(zr)

	// walk through every file in the folder
	filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		// generate tar header
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		// must provide real name
		// (see https://golang.org/src/archive/tar/common.go?#L626)
		name := filepath.ToSlash(file)
		header.Name = name[strings.Index(name, repo):]
		// write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		// if not a dir, write file content
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}
		return nil
	})

	// produce tar
	if err := tw.Close(); err != nil {
		return err
	}
	// produce gzip
	if err := zr.Close(); err != nil {
		return err
	}
	//
	return nil
}

// check for path traversal and correct forward slashes
func validRelPath(p string) bool {
	if p == "" || strings.Contains(p, `\`) || strings.HasPrefix(p, "/") || strings.Contains(p, "../") {
		return false
	}
	return true
}
