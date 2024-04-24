package runner

import (
	"errors"
	"fmt"

	cacidyiov1alpha1 "github.com/cacidy-io/operator/api/v1alpha1"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
)

type Application struct {
	Repository string
	Branch     string
	AuthSecret string
}

func NewApplication(spec *cacidyiov1alpha1.Application) *Application {
	return &Application{
		Repository: spec.Repository,
		Branch:     spec.Branch,
		AuthSecret: spec.AuthSecret,
	}
}

func (app *Application) GetChecksum(auth transport.AuthMethod) (string, error) {
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
			return ref.Hash().String(), nil
		}
	}
	return "", errors.New("checksum not found")
}
