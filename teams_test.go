package main

import (
	"context"
	"net/http"
	"testing"
)

// ---------------------------------------------------------------------------
// isTeam
// ---------------------------------------------------------------------------

func TestIsTeam_OrgNotFound(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	// Returns false before calling GetTeamBySlug (which would log.Fatal on error).
	if isTeam(ctx, client, "mdsol", "any-team") {
		t.Error("isTeam expected false when org is not found")
	}
}

func TestIsTeam_Found(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"login": "mdsol"})
	})
	mux.HandleFunc("/orgs/mdsol/teams/team-alpha", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"id":   1,
			"name": "Team Alpha",
			"slug": "team-alpha",
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	if !isTeam(ctx, client, "mdsol", "team-alpha") {
		t.Error("isTeam expected true for existing team")
	}
}

// ---------------------------------------------------------------------------
// summarizeTeam – smoke test: should not panic and should return non-empty string
// ---------------------------------------------------------------------------

func TestSummarizeTeam_Smoke(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()

	orgID := int64(42)
	teamID := int64(7)

	mux.HandleFunc("/organizations/42/team/7/members", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]string{
			{"login": "user1"},
			{"login": "user2"},
		})
	})
	mux.HandleFunc("/organizations/42/team/7/repos", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"name":        "repo-one",
				"permissions": map[string]bool{"push": true, "pull": true, "admin": false},
				"owner":       map[string]string{"login": "mdsol"},
			},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	teamName := "Team Alpha"
	teamSlug := "team-alpha"
	teamURL := "https://github.com/orgs/mdsol/teams/team-alpha"
	team := newGithubTeam(orgID, teamID, teamName, teamSlug, teamURL)

	summary := summarizeTeam(ctx, client, team)
	if summary == "" {
		t.Error("summarizeTeam returned empty string")
	}
	if len(summary) < 10 {
		t.Errorf("summarizeTeam returned unexpectedly short summary: %q", summary)
	}
}
