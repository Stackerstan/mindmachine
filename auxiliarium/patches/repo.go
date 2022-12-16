package patches

import (
	"bytes"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	dircopy "github.com/otiai10/copy"
	"github.com/sasha-s/go-deadlock"

	"mindmachine/mindmachine"
)

func makeRepo(name string) *Repository {
	var repo Repository
	repo.Name = name
	repo.Data = make(map[mindmachine.S256Hash]Patch)
	repo.Ignore = append(repo.Ignore, ".idea", ".mindmachine", "go.sum")
	repo.mutex = &deadlock.Mutex{}
	return &repo
}

func getRepo(name string) (*Repository, bool) {
	for _, repository := range currentState.data {
		if repository.Name == name {
			return repository, true
		}
	}
	return nil, false
}

func (r *Repository) rootDir() string {
	return srcRootDir() + r.Name + "/"
}

func (r *Repository) tip() string {
	err := os.MkdirAll(r.rootDir()+"TIP", 0777)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	return r.rootDir() + "TIP/"
}

func (r *Repository) temp() string {
	err := os.MkdirAll(r.rootDir()+"temp", 0777)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	return r.rootDir() + "temp/"
}

func (r *Repository) offer(problem mindmachine.S256Hash) string {
	err := os.MkdirAll(r.rootDir()+"offer/"+problem, 0777)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	return r.rootDir() + "offer/" + problem
}

// offerBase is a copy of the tip, created at the time when someone starts work on a new patch offer
func (r *Repository) offerBase(problem mindmachine.S256Hash) string {
	err := os.MkdirAll(r.rootDir()+"offerBase/"+problem, 0777)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	return r.rootDir() + "offerBase/" + problem
}

func (r *Repository) conflicts() string {
	err := os.MkdirAll(r.rootDir()+"conflicts", 0777)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	return r.rootDir() + "conflicts/"
}

func (r *Repository) lock() {
	r.mutex.Lock()
}

func (r *Repository) unlock() {
	r.mutex.Unlock()
}

func (r *Repository) writePatchToTemp(patch *Patch) string {
	patchFile := tempDir() + patch.UID
	err := ioutil.WriteFile(patchFile, patch.Diff, 0777)
	if err != nil {
		mindmachine.LogCLI(err, 0)
	}
	return patchFile
}

func (r *Repository) applyPatch(patch Patch) error {
	//note: only reason for using FS here is I'm not sure about stdin on Windows
	patchFile := r.writePatchToTemp(&patch)
	cmd := exec.Command("git", "apply", "--ignore-whitespace", "--whitespace=nowarn", patchFile)
	cmd.Dir = r.tip()
	var cmdOut bytes.Buffer
	cmd.Stdout = &cmdOut
	err := cmd.Run()
	if err != nil {
		mindmachine.LogCLI("error while executing: "+cmd.String()+" in "+cmd.Dir, 2)
		mindmachine.LogCLI(err.Error(), 1)
		return err
	}
	//Insert patch data
	_ = os.RemoveAll(r.tip() + ".mindmachine")
	err = os.MkdirAll(r.tip()+".mindmachine", 0777)
	if err != nil {
		return err
	}
	patchBytes, err := mindmachine.ToBytes(patch)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(r.tip()+".mindmachine/patch", patchBytes, 0777)
	if err != nil {
		return err
	}
	return nil
}

func (r *Repository) diff(problem mindmachine.S256Hash, base string, patched string) (Patch, error) {
	//verify that the directories exist
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return Patch{}, fmt.Errorf("The directory " + base + " does not exist!")
	}
	if _, err := os.Stat(patched); os.IsNotExist(err) {
		return Patch{}, fmt.Errorf("The directory " + patched + " does not exist!")
	}
	// Remove annoying OSX garbage
	err := filepath.Walk(base,
		func(path string, info fs.FileInfo, err error) error {
			if info.Name() == ".DS_Store" {
				err := os.RemoveAll(path)
				if err != nil {
					mindmachine.LogCLI(err.Error(), 3)
				}
			}
			return nil
		})
	if err != nil {
		mindmachine.LogCLI(err.Error(), 3)
	}
	err = filepath.Walk(patched,
		func(path string, info fs.FileInfo, err error) error {
			if info.Name() == ".DS_Store" {
				err := os.RemoveAll(path)
				if err != nil {
					mindmachine.LogCLI(err.Error(), 3)
				}
			}
			return nil
		})
	if err != nil {
		mindmachine.LogCLI(err.Error(), 3)
	}
	//Remove ignored files but keep a copy in temp dir so they can be replaced later
	//this should include IDE settings etc that people sometimes like to persist.
	_ = os.RemoveAll(r.temp())
	_ = os.MkdirAll(r.temp(), 0777)
	for _, s := range r.Ignore {
		dircopy.Copy(r.offer(problem)+"/"+s, r.temp()+"offer/"+s)
		_ = os.RemoveAll(r.offer(problem) + "/" + s)
		dircopy.Copy(base+"/"+s, r.temp()+"base/"+s)
		_ = os.RemoveAll(base + "/" + s)
	}
	cmd := exec.Command("git", "diff", "--ignore-space-at-eol", base, patched)
	var cmdOut bytes.Buffer
	cmd.Stdout = &cmdOut
	err = cmd.Run()
	if err != nil {
		if err.Error() != "exit status 1" {
			mindmachine.LogCLI(cmd.String(), 4)
			mindmachine.LogCLI(err.Error(), 1)
		}
	}
	clean := strings.ReplaceAll(cmdOut.String(), "b"+base, "b")
	clean = strings.ReplaceAll(clean, "a"+base, "a")
	clean = strings.ReplaceAll(clean, "b"+patched, "b")
	clean = strings.ReplaceAll(clean, "a"+patched, "a")
	patchBytes := []byte(clean)

	patchObject := Patch{
		Diff: patchBytes,
		UID:  mindmachine.Sha256(patchBytes),
	}
	//Put the unwanted files back
	for _, s := range r.Ignore {
		dircopy.Copy(r.temp()+"/offer/"+s, r.offer(problem)+"/"+s)
		dircopy.Copy(base+"/base/"+s, base+"/"+s)
	}
	return patchObject, nil
}

func (r *Repository) validateNoConflicts(patch Patch) error {
	// We need to check if there have been any merged patches since work on this patch was started,
	// and if so, does our work conflict with that of others?
	// First, build the latest tip
	err := r.BuildTip()
	if err != nil {
		return err
	}
	// now we attempt to apply the patch. If there are conflicts, this will tell us.
	err = r.applyPatch(patch)
	if err != nil {
		if err.Error() != "exit status 1" {
			mindmachine.LogCLI(err.Error(), 1)
			return err
		}
		path, conflictErr := r.handleConflicts(patch)
		if conflictErr != nil {
			mindmachine.LogCLI(conflictErr.Error(), 1)
			return err
		}
		return fmt.Errorf("your patch has conflicts, please see the report at " + path)
	}
	return nil
}

func (r *Repository) handleConflicts(patch Patch) (string, error) {
	var path string
	conflicts, err := r.findConflicts(patch)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 1)
		return "", err
	}
	var rejectBytes bytes.Buffer
	for i, reject := range conflicts {
		rejectBytes.WriteString("========== CONFLICT " + fmt.Sprint(i) + " ==========")
		rejectBytes.WriteString(reject)
	}
	path = r.conflicts() + "/" + patch.UID + ".conflicts"
	err = os.WriteFile(path, rejectBytes.Bytes(), 0777)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 1)
		return "", err
	}
	return path, nil
}

func (r *Repository) findConflicts(patch Patch) ([]string, error) {
	var rejects []string
	patchFile := r.writePatchToTemp(&patch)
	cmd := exec.Command("git", "apply", "--reject", "--ignore-whitespace", "--whitespace=nowarn", patchFile)
	cmd.Dir = r.tip()
	err := cmd.Run()
	if err != nil {
		mindmachine.LogCLI(err.Error(), 1)
		mindmachine.LogCLI("error running: "+cmd.String()+" in "+cmd.Dir, 3)
		return []string{}, err
	}
	err = filepath.Walk(r.tip(),
		func(path string, info fs.FileInfo, err error) error {
			if strings.Contains(info.Name(), ".rej") {
				//combine all into one string and output a human readable file
				err = filepath.Walk(r.tip(), func(path string, info fs.FileInfo, err error) error {
					if strings.Contains(info.Name(), ".rej") {
						rejection, err := ioutil.ReadFile(path)
						if err != nil {
							return err
						}
						rejects = append(rejects, string(rejection))
						err = os.Remove(path)
						if err != nil {
							return err
						}
					}
					return nil
				})
				if err != nil {
					return err
				}
				return nil
			}
			return nil
		})
	if err != nil {
		return []string{}, err
	}
	return rejects, nil
}
