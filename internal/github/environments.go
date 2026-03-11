package github

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/go-github/v60/github"
	"golang.org/x/crypto/nacl/box"
)

type EnvVariable struct {
	Name      string
	Value     string
	UpdatedAt time.Time
}

type EnvSecret struct {
	Name      string
	UpdatedAt time.Time
}

func ListEnvVariables(ctx context.Context, client *github.Client, repoID int, envName string) ([]EnvVariable, error) {
	opts := &github.ListOptions{PerPage: 100}
	vars, _, err := client.Actions.ListEnvVariables(ctx, repoID, envName, opts)
	if err != nil {
		return nil, err
	}

	results := make([]EnvVariable, 0, len(vars.Variables))
	for _, v := range vars.Variables {
		results = append(results, EnvVariable{
			Name:      v.Name,
			Value:     v.Value,
			UpdatedAt: v.UpdatedAt.Time,
		})
	}
	return results, nil
}

func CreateEnvVariable(ctx context.Context, client *github.Client, repoID int, envName, name, value string) error {
	v := &github.ActionsVariable{
		Name:  name,
		Value: value,
	}
	_, err := client.Actions.CreateEnvVariable(ctx, repoID, envName, v)
	return err
}

func UpdateEnvVariable(ctx context.Context, client *github.Client, repoID int, envName string, variable *github.ActionsVariable) error {
	_, err := client.Actions.UpdateEnvVariable(ctx, repoID, envName, variable)
	return err
}

func DeleteEnvVariable(ctx context.Context, client *github.Client, repoID int, envName, name string) error {
	_, err := client.Actions.DeleteEnvVariable(ctx, repoID, envName, name)
	return err
}

func ListEnvSecrets(ctx context.Context, client *github.Client, repoID int, envName string) ([]EnvSecret, error) {
	opts := &github.ListOptions{PerPage: 100}
	secrets, _, err := client.Actions.ListEnvSecrets(ctx, repoID, envName, opts)
	if err != nil {
		return nil, err
	}

	results := make([]EnvSecret, 0, len(secrets.Secrets))
	for _, s := range secrets.Secrets {
		results = append(results, EnvSecret{
			Name:      s.Name,
			UpdatedAt: s.UpdatedAt.Time,
		})
	}
	return results, nil
}

func CreateOrUpdateEnvSecret(ctx context.Context, client *github.Client, repoID int, envName, name, value string) error {
	// Get the public key for encryption
	pubKey, _, err := client.Actions.GetEnvPublicKey(ctx, repoID, envName)
	if err != nil {
		return fmt.Errorf("get public key: %w", err)
	}

	encryptedValue, err := encryptSecret(pubKey.GetKey(), value)
	if err != nil {
		return fmt.Errorf("encrypt secret: %w", err)
	}

	secret := &github.EncryptedSecret{
		Name:           name,
		KeyID:          pubKey.GetKeyID(),
		EncryptedValue: encryptedValue,
	}
	_, err = client.Actions.CreateOrUpdateEnvSecret(ctx, repoID, envName, secret)
	return err
}

func DeleteEnvSecret(ctx context.Context, client *github.Client, repoID int, envName, name string) error {
	_, err := client.Actions.DeleteEnvSecret(ctx, repoID, envName, name)
	return err
}

func CreateEnvironment(ctx context.Context, client *github.Client, owner, repo, envName string) error {
	_, _, err := client.Repositories.CreateUpdateEnvironment(ctx, owner, repo, envName, nil)
	return err
}

func DeleteEnvironment(ctx context.Context, client *github.Client, owner, repo, envName string) error {
	_, err := client.Repositories.DeleteEnvironment(ctx, owner, repo, envName)
	return err
}

// encryptSecret encrypts a secret value using the repository's public key (NaCl Sealed Box).
func encryptSecret(publicKeyB64, secretValue string) (string, error) {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return "", fmt.Errorf("decode public key: %w", err)
	}

	if len(publicKeyBytes) != 32 {
		return "", fmt.Errorf("public key must be 32 bytes, got %d", len(publicKeyBytes))
	}

	var recipientKey [32]byte
	copy(recipientKey[:], publicKeyBytes)

	encrypted, err := box.SealAnonymous(nil, []byte(secretValue), &recipientKey, rand.Reader)
	if err != nil {
		return "", fmt.Errorf("seal: %w", err)
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}
