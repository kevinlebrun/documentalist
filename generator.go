package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
)

const (
	EventPush         = "Push"
	EventMergeRequest = "MergeRequest"
)

type DocumentationRequest struct {
	EventName     string
	Repository    string
	ProjectName   string
	ProjectID     int
	Hash          string
	Ref           string
	DefaultBranch string
	MergeRequest  *MergeRequest
}

func (req DocumentationRequest) IsDefaultBranch() bool {
	return req.Ref != "refs/heads/"+req.DefaultBranch
}

type MergeRequest struct {
	ID int
}

type ProjectOptions struct {
	Command []string `json:"command"`
	Path    string   `json:"path"`
	Notify  bool     `json:"notify"`
}

func DocumentGenerator(dest string, in <-chan DocumentationRequest, messages chan<- MergeRequestMessageOptions) {
	for query := range in {
		err, message := generate(dest, query)
		if err != nil {
			log.Printf("Error during generation for %+v, %s\n", query, err)
		}

		if message != nil {
			messages <- *message
		}
	}
}

func generate(dest string, q DocumentationRequest) (error, *MergeRequestMessageOptions) {
	if q.EventName == EventPush && q.IsDefaultBranch() {
		return nil, nil
	}

	revDir := path.Join(dest, q.ProjectName, q.Hash)

	err, dir := prepare(q.Repository, q.ProjectName, q.Hash)
	if err != nil {
		return err, nil
	}

	defer os.RemoveAll(dir)

	options, err := parseProjectOptions(dir)
	if err != nil {
		return err, nil
	}

	if _, err := os.Stat(revDir); os.IsNotExist(err) {
		err = makeDoc(options.Command, dir)
		if err != nil {
			return err, nil
		}

		err = os.MkdirAll(path.Join(dest, q.ProjectName), 0755)
		if err != nil {
			return err, nil
		}

		err = os.Rename(path.Join(dir, options.Path), revDir)
		if err != nil {
			return err, nil
		}
	}

	if q.IsDefaultBranch() {
		masterLink := path.Join(dest, q.ProjectName, q.DefaultBranch)
		os.Remove(masterLink)
		os.Symlink(revDir, masterLink)
	}

	if q.MergeRequest != nil {
		err := os.MkdirAll(path.Join(dest, q.ProjectName, "merge_requests"), 0755)
		if err != nil {
			return err, nil
		}

		MRLink := path.Join(dest, q.ProjectName, "merge_requests", strconv.Itoa(q.MergeRequest.ID))

		var message *MergeRequestMessageOptions
		if _, err := os.Stat(MRLink); os.IsNotExist(err) && options.Notify {
			message = &MergeRequestMessageOptions{
				ProjectID:      q.ProjectID,
				ProjectName:    q.ProjectName,
				MergeRequestID: q.MergeRequest.ID,
				PermalinkPath:  fmt.Sprintf("/%s/merge_requests/%s/", q.ProjectName, strconv.Itoa(q.MergeRequest.ID)),
			}
		}

		os.Remove(MRLink)
		os.Symlink(revDir, MRLink)

		return nil, message
	}

	return nil, nil
}

func prepare(repository, projectName, hash string) (error, string) {
	var err error

	dir := path.Join(os.TempDir(), projectName)

	err = clone(repository, dir)
	if err != nil {
		return err, ""
	}

	err = checkout(hash, dir)
	if err != nil {
		return err, ""
	}

	return nil, dir
}

func parseProjectOptions(dir string) (*ProjectOptions, error) {
	if _, err := os.Stat(path.Join(dir, ".documentalist.json")); err == nil {
		var o ProjectOptions

		data, err := ioutil.ReadFile(path.Join(dir, ".documentalist.json"))
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(data, &o)
		if err != nil {
			return nil, err
		}

		return &o, nil
	}

	// XXX retro compatibility
	return &ProjectOptions{
		Command: []string{"make", "build_doc"},
		Path:    "build/doc",
		Notify:  true,
	}, nil
}

func clone(repository, dir string) error {
	cmd := exec.Command("git", "clone", repository, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func checkout(hash, dir string) error {
	cmd := exec.Command("git", "-C", dir, "checkout", hash)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func makeDoc(command []string, dir string) error {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
