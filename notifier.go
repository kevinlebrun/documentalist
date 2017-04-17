package main

import (
	"fmt"
	"log"

	gitlabAPI "github.com/xanzy/go-gitlab"
)

type MergeRequestMessageOptions struct {
	PermalinkPath  string
	ProjectID      int
	MergeRequestID int
	ProjectName    string
}

func Notifier(client *gitlabAPI.Client, baseURL string, messages <-chan MergeRequestMessageOptions) {
	for m := range messages {
		body := fmt.Sprintf("You can find the documentation here: %s%s", baseURL, m.PermalinkPath)
		o := &gitlabAPI.CreateMergeRequestNoteOptions{
			Body: &body,
		}
		_, _, err := client.Notes.CreateMergeRequestNote(m.ProjectID, m.MergeRequestID, o)
		if err != nil {
			log.Printf("Failed to deliver notification for %q\n%+v\n", m.PermalinkPath, err)
		}
	}
}
