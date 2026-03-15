package github

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/google/go-github/v60/github"
)

// GetFileContent fetches a single file's decoded content from a repo.
// Returns the file content as bytes and any error.
// A 404 error is returned if the file does not exist.
func GetFileContent(ctx context.Context, client *github.Client, owner, repo, path string) ([]byte, error) {
	content, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		return nil, fmt.Errorf("get file %s: %w", path, err)
	}

	if content == nil || content.Content == nil {
		return nil, fmt.Errorf("file %s has no content", path)
	}

	decoded, err := base64.StdEncoding.DecodeString(*content.Content)
	if err != nil {
		return nil, fmt.Errorf("decode file %s: %w", path, err)
	}

	return decoded, nil
}
