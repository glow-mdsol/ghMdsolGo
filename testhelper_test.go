package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/google/go-github/v43/github"
)

// newTestClient returns a GitHub client whose BaseURL points at a fresh
// httptest.Server backed by the given mux.  Call teardown() when done.
func newTestClient(mux *http.ServeMux) (*github.Client, func()) {
	server := httptest.NewServer(mux)
	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	client.UploadURL = u
	return client, server.Close
}

// writeJSON writes v as an application/json response body.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// newGithubTeam builds a *github.Team with the fields needed by tests.
func newGithubTeam(orgID, teamID int64, name, slug, htmlURL string) *github.Team {
	return &github.Team{
		ID:      github.Int64(teamID),
		Name:    github.String(name),
		Slug:    github.String(slug),
		HTMLURL: github.String(htmlURL),
		Organization: &github.Organization{
			ID: github.Int64(orgID),
		},
	}
}
