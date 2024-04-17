package controller

import (
	"errors"
	"fmt"

	cacidyiov1alpha1 "cacidy.io/runner/api/v1alpha1"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
)

const (
	appCheckumInterval = 10
)

type application struct {
	Repository       string
	Branch           string
	RepositorySecret string
}

func newApplication(spec *cacidyiov1alpha1.RunnerSpecApplication) *application {
	return &application{
		Repository:       spec.Repository,
		Branch:           spec.Branch,
		RepositorySecret: spec.RepositorySecret,
	}
}

func (app *application) getChecksum(auth transport.AuthMethod) (string, error) {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{app.Repository},
	})
	refs, err := remote.List(&git.ListOptions{Auth: auth})
	if err != nil {
		return "", fmt.Errorf("failed getting the application checksum: %s", err)
	}
	for _, ref := range refs {
		if ref.Name().String() == fmt.Sprintf("refs/heads/%s", app.Branch) {
			return ref.Hash().String()[:7], nil
		}
	}
	return "", errors.New("checksum not found")
}
