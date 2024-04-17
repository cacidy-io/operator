package controller

import (
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	v1 "k8s.io/api/core/v1"
)

func getAuthSecret() (*v1.Secret, error) {
	var authSecret *v1.Secret
	return authSecret, nil
}

func gitAuth() (transport.AuthMethod, error) {
	authSecret, err := getAuthSecret()
	if err != nil {
		return nil, err
	}
	if authSecret != nil {
		return &http.BasicAuth{
			Username: string(authSecret.Data["username"]),
			Password: string(authSecret.Data["password"]),
		}, nil
	}
	return nil, nil
}
