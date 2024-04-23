package util

import (
	"context"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetAuthSecret(cli client.Client, ctx context.Context, name, namespace string) (*v1.Secret, error) {
	authSecret := &v1.Secret{}
	if name != "" {
		key := types.NamespacedName{Name: name, Namespace: namespace}
		if err := cli.Get(ctx, key, authSecret); err != nil {
			return authSecret, err
		}
	}
	return authSecret, nil
}

func GitAuth(authSecret *v1.Secret) transport.AuthMethod {
	if authSecret != nil {
		return &http.BasicAuth{
			Username: string(authSecret.Data["username"]),
			Password: string(authSecret.Data["password"]),
		}
	}
	return nil
}
