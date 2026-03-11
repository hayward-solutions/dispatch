package github

import (
	"testing"

	gh "github.com/google/go-github/v60/github"
)

func strPtr(s string) *string { return &s }
func intPtr(i int64) *int64   { return &i }
func boolPtr(b bool) *bool    { return &b }

func TestRepoFromGitHub(t *testing.T) {
	r := &gh.Repository{
		ID:          intPtr(123),
		Name:        strPtr("my-repo"),
		FullName:    strPtr("owner/my-repo"),
		Description: strPtr("A test repo"),
		Private:     boolPtr(true),
		HTMLURL:     strPtr("https://github.com/owner/my-repo"),
		Owner: &gh.User{
			Login: strPtr("owner"),
		},
	}

	result := repoFromGitHub(r)

	if result.ID != 123 {
		t.Errorf("ID: expected 123, got %d", result.ID)
	}
	if result.Name != "my-repo" {
		t.Errorf("Name: expected 'my-repo', got %q", result.Name)
	}
	if result.Owner != "owner" {
		t.Errorf("Owner: expected 'owner', got %q", result.Owner)
	}
	if result.FullName != "owner/my-repo" {
		t.Errorf("FullName: expected 'owner/my-repo', got %q", result.FullName)
	}
	if result.Description != "A test repo" {
		t.Errorf("Description: expected 'A test repo', got %q", result.Description)
	}
	if !result.Private {
		t.Error("expected Private=true")
	}
	if result.HTMLURL != "https://github.com/owner/my-repo" {
		t.Errorf("HTMLURL: unexpected value %q", result.HTMLURL)
	}
}

func TestRepoFromGitHub_NilFields(t *testing.T) {
	r := &gh.Repository{}

	result := repoFromGitHub(r)

	if result.ID != 0 {
		t.Errorf("expected 0 ID, got %d", result.ID)
	}
	if result.Name != "" {
		t.Errorf("expected empty name, got %q", result.Name)
	}
	if result.Owner != "" {
		t.Errorf("expected empty owner, got %q", result.Owner)
	}
}

func TestRepoFromGitHub_NilOwner(t *testing.T) {
	r := &gh.Repository{
		ID:   intPtr(1),
		Name: strPtr("test"),
	}

	result := repoFromGitHub(r)

	if result.Owner != "" {
		t.Errorf("expected empty owner for nil Owner, got %q", result.Owner)
	}
}
