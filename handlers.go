package main

import (
	"gopkg.in/go-playground/webhooks.v2"
	"gopkg.in/go-playground/webhooks.v2/gitlab"
)

func HandlePush(in chan<- DocumentationRequest) func(interface{}, webhooks.Header) {
	return func(payload interface{}, header webhooks.Header) {
		pl := payload.(gitlab.PushEventPayload)

		req := DocumentationRequest{
			EventName:     EventPush,
			Repository:    pl.Repository.URL,
			ProjectName:   pl.Project.Name,
			ProjectID:     pl.ProjectID,
			Hash:          pl.After,
			Ref:           pl.Ref,
			DefaultBranch: pl.Project.DefaultBranch,
			MergeRequest:  nil,
		}

		in <- req
	}
}

func HandleMergeRequest(in chan<- DocumentationRequest) func(interface{}, webhooks.Header) {
	return func(payload interface{}, header webhooks.Header) {
		pl := payload.(gitlab.MergeRequestEventPayload)

		req := DocumentationRequest{
			EventName:     EventMergeRequest,
			Repository:    pl.ObjectAttributes.Source.GitSSHURL,
			ProjectName:   pl.ObjectAttributes.Source.Name,
			ProjectID:     pl.ObjectAttributes.TargetProjectID,
			Hash:          pl.ObjectAttributes.LastCommit.ID,
			Ref:           pl.ObjectAttributes.SourceBranch,
			DefaultBranch: pl.ObjectAttributes.Source.DefaultBranch,
			MergeRequest: &MergeRequest{
				ID: pl.ObjectAttributes.ID,
			},
		}

		in <- req
	}
}
