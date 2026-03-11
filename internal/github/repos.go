package github

import (
	"context"

	"github.com/google/go-github/v60/github"
)

type Repo struct {
	ID          int64
	Owner       string
	Name        string
	FullName    string
	Description string
	Private     bool
	HTMLURL     string
}

func ListUserRepos(ctx context.Context, client *github.Client, page, perPage int) ([]Repo, bool, error) {
	opts := &github.RepositoryListOptions{
		Visibility:  "all",
		Affiliation: "owner,collaborator,organization_member",
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: perPage,
		},
	}

	repos, resp, err := client.Repositories.List(ctx, "", opts)
	if err != nil {
		return nil, false, err
	}

	result := make([]Repo, 0, len(repos))
	for _, r := range repos {
		result = append(result, repoFromGitHub(r))
	}

	hasMore := resp.NextPage > 0
	return result, hasMore, nil
}

func SearchRepos(ctx context.Context, client *github.Client, query string, page, perPage int) ([]Repo, bool, error) {
	opts := &github.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: perPage,
		},
	}

	result, resp, err := client.Search.Repositories(ctx, query, opts)
	if err != nil {
		return nil, false, err
	}

	repos := make([]Repo, 0, len(result.Repositories))
	for _, r := range result.Repositories {
		repos = append(repos, repoFromGitHub(r))
	}

	hasMore := resp.NextPage > 0
	return repos, hasMore, nil
}

func GetRepo(ctx context.Context, client *github.Client, owner, name string) (*Repo, error) {
	r, _, err := client.Repositories.Get(ctx, owner, name)
	if err != nil {
		return nil, err
	}
	repo := repoFromGitHub(r)
	return &repo, nil
}

func repoFromGitHub(r *github.Repository) Repo {
	return Repo{
		ID:          r.GetID(),
		Owner:       r.GetOwner().GetLogin(),
		Name:        r.GetName(),
		FullName:    r.GetFullName(),
		Description: r.GetDescription(),
		Private:     r.GetPrivate(),
		HTMLURL:     r.GetHTMLURL(),
	}
}
