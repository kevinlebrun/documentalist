package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
)

type DocumentationRequest struct {
	Repository   string
	ProjectName  string
	ProjectID    int
	Hash         string
	Ref          string
	MergeRequest *MergeRequest
}

type MergeRequest struct {
	ID int
}

func DocumentGenerator(dest string, in <-chan DocumentationRequest, out chan<- MergeRequestMessageOptions) {
	for q := range in {
		err, message := generate(dest, q)
		if err != nil {
			panic(err)
		}

		if message != nil {
			out <- *message
		}
	}
}

func generate(dest string, q DocumentationRequest) (error, *MergeRequestMessageOptions) {
	revDir := path.Join(dest, q.ProjectName, q.Hash)

	if _, err := os.Stat(revDir); os.IsNotExist(err) {
		err, dir := prepare(q.Repository, q.ProjectName, q.Hash)
		if err != nil {
			return err, nil
		}

		defer os.RemoveAll(dir)

		err = makeDoc(dir)
		if err != nil {
			return err, nil
		}

		err = os.MkdirAll(path.Join(dest, q.ProjectName), 0755)
		if err != nil {
			return err, nil
		}

		// TODO get from .documentalist.yml
		err = os.Rename(path.Join(dir, "build", "doc"), revDir)
		if err != nil {
			return err, nil
		}
	}

	if q.Ref == "refs/heads/master" {
		masterLink := path.Join(dest, q.ProjectName, "master")
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
		if _, err := os.Stat(MRLink); os.IsNotExist(err) {
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

func makeDoc(dir string) error {
	// TODO get from .documentalist.yml
	cmd := exec.Command("make", "build_doc", "-C", dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
