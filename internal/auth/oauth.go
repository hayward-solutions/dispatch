package auth

import (
	"context"
	"fmt"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
)

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

func NewOAuthConfig(clientID, clientSecret, baseURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  baseURL + "/auth/github/callback",
		Scopes:       []string{"repo", "workflow"},
		Endpoint:     githuboauth.Endpoint,
	}
}

type GitHubUser struct {
	ID        int64
	Login     string
	Name      string
	AvatarURL string
}

func FetchGitHubUser(ctx context.Context, token *oauth2.Token) (*GitHubUser, error) {
	client := github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(token)))

	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("fetch github user: %w", err)
	}

	return &GitHubUser{
		ID:        user.GetID(),
		Login:     user.GetLogin(),
		Name:      user.GetName(),
		AvatarURL: user.GetAvatarURL(),
	}, nil
}
