package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-github/v84/github"
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

// ---------------------------------------------------------------------------
// getTeamByName – happy path only (error path calls log.Fatal)
// ---------------------------------------------------------------------------

func TestGetTeamByName(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol/teams/team-alpha", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"id":   1,
			"name": "Team Alpha",
			"slug": "team-alpha",
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	team := getTeamByName(ctx, client, "mdsol", "Team Alpha")
	if team == nil {
		t.Fatal("expected non-nil team")
	}
	if team.GetSlug() != "team-alpha" {
		t.Errorf("slug = %q, want %q", team.GetSlug(), "team-alpha")
	}
}

// ---------------------------------------------------------------------------
// checkAndAddMember – already-member path (avoids log.Fatal and prompt)
// ---------------------------------------------------------------------------

func TestCheckAndAddMember_AlreadyMember(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/1/team/2/memberships/testuser", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"state": "active",
			"role":  "member",
			"url":   "https://api.github.com/organizations/1/team/2/memberships/testuser",
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	team := newGithubTeam(1, 2, "Test Team", "test-team", "https://github.com")
	login := "testuser"
	user := &github.User{Login: &login}
	// Should log "already a member" and return – no panic, no Fatal.
	checkAndAddMember(ctx, client, team, user)
}

// ---------------------------------------------------------------------------
// summarizeTeam – all permission branch coverage
// ---------------------------------------------------------------------------

func TestSummarizeTeam_AllPermissions(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()

	orgID := int64(10)
	teamID := int64(20)

	mux.HandleFunc("/organizations/10/team/20/members", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]string{{"login": "user1"}})
	})
	mux.HandleFunc("/organizations/10/team/20/repos", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"name":        "admin-repo",
				"permissions": map[string]bool{"admin": true, "push": true, "pull": true},
				"owner":       map[string]string{"login": "mdsol"},
			},
			{
				"name":        "maintain-repo",
				"permissions": map[string]bool{"admin": false, "maintain": true, "push": false, "pull": true},
				"owner":       map[string]string{"login": "mdsol"},
			},
			{
				"name":        "triage-repo",
				"permissions": map[string]bool{"admin": false, "maintain": false, "push": false, "triage": true, "pull": true},
				"owner":       map[string]string{"login": "mdsol"},
			},
			{
				"name":        "read-repo",
				"permissions": map[string]bool{"admin": false, "maintain": false, "push": false, "triage": false, "pull": true},
				"owner":       map[string]string{"login": "mdsol"},
			},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	team := newGithubTeam(orgID, teamID, "Perm Team", "perm-team", "https://github.com")
	summary := summarizeTeam(ctx, client, team)
	if summary == "" {
		t.Error("summarizeTeam returned empty string")
	}
}
