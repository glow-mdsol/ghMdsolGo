package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// normalizePermission
// ---------------------------------------------------------------------------

func TestNormalizePermission(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"pull", "read"},
		{"push", "write"},
		{"admin", "admin"},
		{"maintain", "maintain"},
		{"triage", "triage"},
		{"read", "read"},
		{"", ""},
	}
	for _, tc := range cases {
		got := normalizePermission(tc.raw)
		if got != tc.want {
			t.Errorf("normalizePermission(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// permissionLevel
// ---------------------------------------------------------------------------

func TestPermissionLevel(t *testing.T) {
	cases := []struct {
		perm  string
		level int
	}{
		{"admin", 5},
		{"maintain", 4},
		{"write", 3},
		{"triage", 2},
		{"read", 1},
		{"", 0},
		{"unknown", 0},
	}
	for _, tc := range cases {
		got := permissionLevel(tc.perm)
		if got != tc.level {
			t.Errorf("permissionLevel(%q) = %d, want %d", tc.perm, got, tc.level)
		}
	}
}

func TestPermissionLevelOrdering(t *testing.T) {
	ordered := []string{"admin", "maintain", "write", "triage", "read", ""}
	for i := 0; i < len(ordered)-1; i++ {
		hi, lo := ordered[i], ordered[i+1]
		if permissionLevel(hi) <= permissionLevel(lo) {
			t.Errorf("expected permissionLevel(%q) > permissionLevel(%q)", hi, lo)
		}
	}
}

// ---------------------------------------------------------------------------
// findMissingRepos
// ---------------------------------------------------------------------------

func TestFindMissingRepos_AllPresent(t *testing.T) {
	repoTeamsMap := map[string][]teamInfo{
		"repo1": {{slug: "team-a"}, {slug: "team-b"}},
		"repo2": {{slug: "team-a"}, {slug: "team-b"}},
	}
	missing := findMissingRepos("team-a", repoTeamsMap)
	if len(missing) != 0 {
		t.Errorf("findMissingRepos(team-a) = %v, want []", missing)
	}
}

func TestFindMissingRepos_SomeMissing(t *testing.T) {
	repoTeamsMap := map[string][]teamInfo{
		"repo1": {{slug: "team-a"}, {slug: "team-b"}},
		"repo2": {{slug: "team-b"}},
		"repo3": {{slug: "team-a"}, {slug: "team-b"}, {slug: "team-c"}},
	}
	// team-a is missing from repo2 only
	missing := findMissingRepos("team-a", repoTeamsMap)
	sort.Strings(missing)
	if len(missing) != 1 || missing[0] != "repo2" {
		t.Errorf("findMissingRepos(team-a) = %v, want [repo2]", missing)
	}
	// team-c is missing from repo1 and repo2
	missing = findMissingRepos("team-c", repoTeamsMap)
	sort.Strings(missing)
	if len(missing) != 2 || missing[0] != "repo1" || missing[1] != "repo2" {
		t.Errorf("findMissingRepos(team-c) = %v, want [repo1 repo2]", missing)
	}
}

// ---------------------------------------------------------------------------
// isRepository
// ---------------------------------------------------------------------------

func TestIsRepository_EmailSlug(t *testing.T) {
	// Slugs containing "@" are never repositories – short-circuit, no API call.
	ctx := context.Background()
	mux := http.NewServeMux()
	client, teardown := newTestClient(mux)
	defer teardown()

	if isRepository(ctx, client, "mdsol", "user@example.com") {
		t.Error("isRepository with email slug should return false")
	}
}

func TestIsRepository_Found(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"login": "mdsol"})
	})
	mux.HandleFunc("/repos/mdsol/my-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "my-repo", "owner": map[string]string{"login": "mdsol"}})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	if !isRepository(ctx, client, "mdsol", "my-repo") {
		t.Error("isRepository expected true for existing repo")
	}
}

func TestIsRepository_NotFound(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"login": "mdsol"})
	})
	mux.HandleFunc("/repos/mdsol/missing-repo", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	if isRepository(ctx, client, "mdsol", "missing-repo") {
		t.Error("isRepository expected false for missing repo")
	}
}

func TestIsRepository_OrgNotFound(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	if isRepository(ctx, client, "mdsol", "some-repo") {
		t.Error("isRepository expected false when org not found")
	}
}

// ---------------------------------------------------------------------------
// checkRepository
// ---------------------------------------------------------------------------

func TestCheckRepository_Exists(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"name":        "my-repo",
			"description": "A test repo",
			"owner":       map[string]string{"login": "mdsol"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	info, err := checkRepository(ctx, client, "mdsol", "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.name != "my-repo" {
		t.Errorf("name = %q, want %q", info.name, "my-repo")
	}
	if info.owner != "mdsol" {
		t.Errorf("owner = %q, want %q", info.owner, "mdsol")
	}
	if info.description != "A test repo" {
		t.Errorf("description = %q, want %q", info.description, "A test repo")
	}
}

func TestCheckRepository_NoDescription(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/bare-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"name":  "bare-repo",
			"owner": map[string]string{"login": "mdsol"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	info, err := checkRepository(ctx, client, "mdsol", "bare-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.description != "" {
		t.Errorf("description = %q, want empty", info.description)
	}
}

func TestCheckRepository_Error(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/missing", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	_, err := checkRepository(ctx, client, "mdsol", "missing")
	if err == nil {
		t.Error("expected error for missing repo, got nil")
	}
}

// ---------------------------------------------------------------------------
// getRepositoryTeams
// ---------------------------------------------------------------------------

func TestGetRepositoryTeams(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "my-repo", "owner": map[string]string{"login": "mdsol"}})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"name":        "Team Alpha",
				"slug":        "team-alpha",
				"html_url":    "https://github.com/orgs/mdsol/teams/team-alpha",
				"permission":  "pull",
				"description": "Alpha team",
			},
			{
				"name":       "Team Beta",
				"slug":       "team-beta",
				"html_url":   "https://github.com/orgs/mdsol/teams/team-beta",
				"permission": "push",
			},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	teams, err := getRepositoryTeams(ctx, client, "mdsol", "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(teams))
	}
	if teams[0].slug != "team-alpha" {
		t.Errorf("slug = %q, want %q", teams[0].slug, "team-alpha")
	}
	if teams[0].access != "pull" {
		t.Errorf("access = %q, want %q", teams[0].access, "pull")
	}
	if teams[0].description != "Alpha team" {
		t.Errorf("description = %q, want %q", teams[0].description, "Alpha team")
	}
	if teams[1].slug != "team-beta" {
		t.Errorf("slug = %q, want %q", teams[1].slug, "team-beta")
	}
}

func TestGetRepositoryTeams_Empty(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/empty-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "empty-repo", "owner": map[string]string{"login": "mdsol"}})
	})
	mux.HandleFunc("/repos/mdsol/empty-repo/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	teams, err := getRepositoryTeams(ctx, client, "mdsol", "empty-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(teams) != 0 {
		t.Errorf("expected 0 teams, got %d", len(teams))
	}
}

func TestGetRepositoryTeams_RepoNotFound(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/missing", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	_, err := getRepositoryTeams(ctx, client, "mdsol", "missing")
	if err == nil {
		t.Error("expected error for missing repo, got nil")
	}
}

// ---------------------------------------------------------------------------
// findTeamsWithAccessAnalysis
// ---------------------------------------------------------------------------

func TestFindTeamsWithAccessAnalysis_EmptyRepos(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	client, teardown := newTestClient(mux)
	defer teardown()

	_, err := findTeamsWithAccessAnalysis(ctx, client, "mdsol", nil)
	if err == nil {
		t.Error("expected error for empty repo list, got nil")
	}
}

func TestFindTeamsWithAccessAnalysis_AllExact(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()

	// Both repos expose the same two teams → both are exact matches.
	for _, repo := range []string{"repo1", "repo2"} {
		repo := repo
		mux.HandleFunc("/repos/mdsol/"+repo, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{"name": repo, "owner": map[string]string{"login": "mdsol"}})
		})
		mux.HandleFunc("/repos/mdsol/"+repo+"/teams", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []map[string]interface{}{
				{"name": "Team Alpha", "slug": "team-alpha", "html_url": "https://github.com", "permission": "push"},
				{"name": "Team Beta", "slug": "team-beta", "html_url": "https://github.com", "permission": "pull"},
			})
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	result, err := findTeamsWithAccessAnalysis(ctx, client, "mdsol", []string{"repo1", "repo2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.exactMatches) != 2 {
		t.Errorf("exactMatches = %d, want 2", len(result.exactMatches))
	}
	if len(result.closeMatches) != 0 {
		t.Errorf("closeMatches = %d, want 0", len(result.closeMatches))
	}
}

func TestFindTeamsWithAccessAnalysis_CloseMatch(t *testing.T) {
	// repo1 + repo2: team-alpha + team-beta  (team-beta = 100%, team-alpha = 67%)
	// repo3:         team-beta only
	ctx := context.Background()
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/mdsol/repo1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "repo1", "owner": map[string]string{"login": "mdsol"}})
	})
	mux.HandleFunc("/repos/mdsol/repo1/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "Team Alpha", "slug": "team-alpha", "html_url": "https://github.com", "permission": "push"},
			{"name": "Team Beta", "slug": "team-beta", "html_url": "https://github.com", "permission": "pull"},
		})
	})
	mux.HandleFunc("/repos/mdsol/repo2", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "repo2", "owner": map[string]string{"login": "mdsol"}})
	})
	mux.HandleFunc("/repos/mdsol/repo2/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "Team Alpha", "slug": "team-alpha", "html_url": "https://github.com", "permission": "push"},
			{"name": "Team Beta", "slug": "team-beta", "html_url": "https://github.com", "permission": "pull"},
		})
	})
	mux.HandleFunc("/repos/mdsol/repo3", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "repo3", "owner": map[string]string{"login": "mdsol"}})
	})
	mux.HandleFunc("/repos/mdsol/repo3/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "Team Beta", "slug": "team-beta", "html_url": "https://github.com", "permission": "pull"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	result, err := findTeamsWithAccessAnalysis(ctx, client, "mdsol", []string{"repo1", "repo2", "repo3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.exactMatches) != 1 {
		t.Fatalf("exactMatches = %d, want 1", len(result.exactMatches))
	}
	if result.exactMatches[0].slug != "team-beta" {
		t.Errorf("exact match = %q, want %q", result.exactMatches[0].slug, "team-beta")
	}

	if len(result.closeMatches) != 1 {
		t.Fatalf("closeMatches = %d, want 1", len(result.closeMatches))
	}
	if result.closeMatches[0].team.slug != "team-alpha" {
		t.Errorf("close match = %q, want %q", result.closeMatches[0].team.slug, "team-alpha")
	}
	if result.closeMatches[0].accessCount != 2 {
		t.Errorf("accessCount = %d, want 2", result.closeMatches[0].accessCount)
	}
	missing := result.closeMatches[0].missingRepos
	if len(missing) != 1 || missing[0] != "repo3" {
		t.Errorf("missingRepos = %v, want [repo3]", missing)
	}
}

func TestFindTeamsWithAccessAnalysis_NoOverlap(t *testing.T) {
	// repo1 has team-a, repo2 has team-b → no exact or close matches
	ctx := context.Background()
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/mdsol/repo1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "repo1", "owner": map[string]string{"login": "mdsol"}})
	})
	mux.HandleFunc("/repos/mdsol/repo1/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "Team A", "slug": "team-a", "html_url": "https://github.com", "permission": "push"},
		})
	})
	mux.HandleFunc("/repos/mdsol/repo2", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "repo2", "owner": map[string]string{"login": "mdsol"}})
	})
	mux.HandleFunc("/repos/mdsol/repo2/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "Team B", "slug": "team-b", "html_url": "https://github.com", "permission": "pull"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	result, err := findTeamsWithAccessAnalysis(ctx, client, "mdsol", []string{"repo1", "repo2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Each team covers only 50%, which is not strictly >50%, so no close matches.
	if len(result.exactMatches) != 0 {
		t.Errorf("exactMatches = %d, want 0", len(result.exactMatches))
	}
	if len(result.closeMatches) != 0 {
		t.Errorf("closeMatches = %d, want 0", len(result.closeMatches))
	}
}

// ---------------------------------------------------------------------------
// findTeamsWithAccessToAllRepos – delegation wrapper
// ---------------------------------------------------------------------------

func TestFindTeamsWithAccessToAllRepos_Delegates(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	for _, repo := range []string{"r1", "r2"} {
		repo := repo
		mux.HandleFunc("/repos/mdsol/"+repo, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{"name": repo, "owner": map[string]string{"login": "mdsol"}})
		})
		mux.HandleFunc("/repos/mdsol/"+repo+"/teams", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []map[string]interface{}{
				{"name": "Team X", "slug": "team-x", "html_url": "https://github.com", "permission": "push"},
			})
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	teams, err := findTeamsWithAccessToAllRepos(ctx, client, "mdsol", []string{"r1", "r2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(teams) != 1 || teams[0].slug != "team-x" {
		t.Errorf("teams = %v, want [{slug:team-x}]", teams)
	}
}

// ---------------------------------------------------------------------------
// createRepository
// ---------------------------------------------------------------------------

func TestCreateRepository_AlreadyExists(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/existing-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "existing-repo", "owner": map[string]string{"login": "mdsol"}})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	info := repositoryInfo{owner: "mdsol", name: "existing-repo", description: "test"}
	result, err := createRepository(ctx, client, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when repo already exists")
	}
}

func TestCreateRepository_GetError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/some-repo", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	info := repositoryInfo{owner: "mdsol", name: "some-repo"}
	_, err := createRepository(ctx, client, info)
	if err == nil {
		t.Error("expected error when GET fails, got nil")
	}
}

// ---------------------------------------------------------------------------
// enableVulnerabilityAlerts
// ---------------------------------------------------------------------------

func TestEnableVulnerabilityAlerts_AlreadyEnabled(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/vulnerability-alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNoContent) // 204 → enabled=true
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	enabled, err := enableVulnerabilityAlerts(ctx, client, "mdsol", "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enabled {
		t.Error("expected false when alerts were already enabled")
	}
}

func TestEnableVulnerabilityAlerts_EnablesNew(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/vulnerability-alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// go-github v43 treats 404 as "not enabled" (returns false, nil)
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}
		// PUT – enable them
		w.WriteHeader(http.StatusNoContent)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	enabled, err := enableVulnerabilityAlerts(ctx, client, "mdsol", "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enabled {
		t.Error("expected true when alerts were newly enabled")
	}
}

func TestEnableVulnerabilityAlerts_GetError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/missing-repo/vulnerability-alerts", func(w http.ResponseWriter, r *http.Request) {
		// 500 causes a genuine GET-level error (not the 404 "not enabled" path)
		http.Error(w, `{"message":"Internal Server Error"}`, http.StatusInternalServerError)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	_, err := enableVulnerabilityAlerts(ctx, client, "mdsol", "missing-repo")
	if err == nil {
		t.Error("expected error when repo not found, got nil")
	}
}

// ---------------------------------------------------------------------------
// listRepositoryCollaborators
// ---------------------------------------------------------------------------

func TestListRepositoryCollaborators_Empty(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := listRepositoryCollaborators(ctx, client, "mdsol", "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListRepositoryCollaborators_WithCollaborator(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"login":    "user1",
				"html_url": "https://github.com/user1",
				"permissions": map[string]bool{
					"admin": false, "maintain": false,
					"push": true, "triage": false, "pull": true,
				},
			},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/events", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators/user1/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"permission": "write",
			"user":       map[string]string{"login": "user1"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := listRepositoryCollaborators(ctx, client, "mdsol", "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListRepositoryCollaborators_APIError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := listRepositoryCollaborators(ctx, client, "mdsol", "my-repo")
	if err == nil {
		t.Error("expected error when API returns 403, got nil")
	}
}

// ---------------------------------------------------------------------------
// addUserAsRepoCollaborator
// ---------------------------------------------------------------------------

func TestAddUserAsRepoCollaborator_NewInvitation(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{}) // no existing collaborators
	})
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators/newuser", func(w http.ResponseWriter, r *http.Request) {
		// PUT adds the collaborator → 201 invitation created
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]interface{}{"id": 1})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := addUserAsRepoCollaborator(ctx, client, "mdsol", "my-repo", "newuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddUserAsRepoCollaborator_AlreadyHasAccess(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators/existinguser", func(w http.ResponseWriter, r *http.Request) {
		// PUT → 204 means user already had access
		w.WriteHeader(http.StatusNoContent)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := addUserAsRepoCollaborator(ctx, client, "mdsol", "my-repo", "existinguser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddUserAsRepoCollaborator_OtherAdminUnknownDate(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		// Another user is already an admin
		writeJSON(w, []map[string]interface{}{
			{
				"login": "other-admin",
				"permissions": map[string]bool{
					"admin": true, "push": true, "pull": true,
				},
			},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/events", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators/newuser", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]interface{}{"id": 1})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := addUserAsRepoCollaborator(ctx, client, "mdsol", "my-repo", "newuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddUserAsRepoCollaborator_APIError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := addUserAsRepoCollaborator(ctx, client, "mdsol", "my-repo", "newuser")
	if err == nil {
		t.Error("expected error when ListCollaborators fails, got nil")
	}
}

// ---------------------------------------------------------------------------
// findAndReportTeamsWithAccessToAllRepos
// ---------------------------------------------------------------------------

func TestFindAndReportTeamsWithAccessToAllRepos_ExactMatch(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	for _, repo := range []string{"r1", "r2"} {
		repo := repo
		mux.HandleFunc("/repos/mdsol/"+repo, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{"name": repo, "owner": map[string]string{"login": "mdsol"}})
		})
		mux.HandleFunc("/repos/mdsol/"+repo+"/teams", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []map[string]interface{}{
				{"name": "All Access Team", "slug": "all-access-team", "html_url": "https://github.com", "permission": "push"},
			})
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	// Must not panic/error; side effects are only stdout logging.
	findAndReportTeamsWithAccessToAllRepos(ctx, client, "mdsol", []string{"r1", "r2"})
}

func TestFindAndReportTeamsWithAccessToAllRepos_NoMatch(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	for i, repo := range []string{"r1", "r2"} {
		repo := repo
		i := i
		slug := fmt.Sprintf("team-%d", i)
		mux.HandleFunc("/repos/mdsol/"+repo, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{"name": repo, "owner": map[string]string{"login": "mdsol"}})
		})
		mux.HandleFunc("/repos/mdsol/"+repo+"/teams", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []map[string]interface{}{
				{"name": "Team " + slug, "slug": slug, "html_url": "https://github.com", "permission": "push"},
			})
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	findAndReportTeamsWithAccessToAllRepos(ctx, client, "mdsol", []string{"r1", "r2"})
}

// ---------------------------------------------------------------------------
// listRepositoryCollaborators – invitation and admin paths
// ---------------------------------------------------------------------------

func TestListRepositoryCollaborators_WithInvitation(t *testing.T) {
	// Collaborator found via invitation (added time from invitation source).
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"login":    "user1",
				"html_url": "https://github.com/user1",
				"permissions": map[string]bool{
					"admin": false, "maintain": false,
					"push": true, "triage": false, "pull": true,
				},
			},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"id":         1,
				"invitee":    map[string]string{"login": "user1"},
				"created_at": "2023-01-01T00:00:00Z",
			},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/events", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators/user1/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"permission": "write",
			"user":       map[string]string{"login": "user1"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := listRepositoryCollaborators(ctx, client, "mdsol", "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListRepositoryCollaborators_AdminOldInvitation(t *testing.T) {
	// Admin user with a very old invitation → "Admin access granted >24 hours ago" warning.
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"login":    "admin1",
				"html_url": "https://github.com/admin1",
				"permissions": map[string]bool{
					"admin": true, "maintain": false,
					"push": true, "triage": false, "pull": true,
				},
			},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"id":         1,
				"invitee":    map[string]string{"login": "admin1"},
				"created_at": "2020-01-01T00:00:00Z", // very old → >24h warning
			},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/events", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators/admin1/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"permission": "admin",
			"user":       map[string]string{"login": "admin1"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := listRepositoryCollaborators(ctx, client, "mdsol", "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// addUserAsRepoCollaborator – target user already has admin access
// ---------------------------------------------------------------------------

func TestAddUserAsRepoCollaborator_TargetUserAlreadyAdmin(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	// The user being added already appears in the collaborators list as admin.
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"login": "newuser",
				"permissions": map[string]bool{
					"admin": true, "maintain": false,
					"push": true, "triage": false, "pull": true,
				},
			},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators/newuser/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"permission": "admin",
			"user":       map[string]string{"login": "newuser"},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := addUserAsRepoCollaborator(ctx, client, "mdsol", "my-repo", "newuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// getRepositoryTeams – ListTeams API error path
// ---------------------------------------------------------------------------

func TestGetRepositoryTeams_TeamsAPIError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/error-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "error-repo", "owner": map[string]string{"login": "mdsol"}})
	})
	mux.HandleFunc("/repos/mdsol/error-repo/teams", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	_, err := getRepositoryTeams(ctx, client, "mdsol", "error-repo")
	if err == nil {
		t.Error("expected error when ListTeams fails, got nil")
	}
}

// ---------------------------------------------------------------------------
// findTeamsWithAccessAnalysis – error and partial-error paths
// ---------------------------------------------------------------------------

func TestFindTeamsWithAccessAnalysis_AllReposFail(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/bad1", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/repos/mdsol/bad2", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	_, err := findTeamsWithAccessAnalysis(ctx, client, "mdsol", []string{"bad1", "bad2"})
	if err == nil {
		t.Error("expected error when all repos fail, got nil")
	}
}

func TestFindTeamsWithAccessAnalysis_PartialRepoError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/good-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "good-repo", "owner": map[string]string{"login": "mdsol"}})
	})
	mux.HandleFunc("/repos/mdsol/good-repo/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "Team A", "slug": "team-a", "html_url": "https://github.com", "permission": "push"},
		})
	})
	mux.HandleFunc("/repos/mdsol/bad-repo", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	result, err := findTeamsWithAccessAnalysis(ctx, client, "mdsol", []string{"good-repo", "bad-repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// team-a has access to 1/1 accessible repos → exact match.
	if len(result.exactMatches) != 1 {
		t.Errorf("exactMatches = %d, want 1", len(result.exactMatches))
	}
}

// ---------------------------------------------------------------------------
// addUserAsRepoCollaborator – other admin found in recent invitation (hasOldAdmin)
// ---------------------------------------------------------------------------

func TestAddUserAsRepoCollaborator_OtherAdminViaInvitation(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"login": "other-admin",
				"permissions": map[string]bool{
					"admin": true, "push": true, "pull": true,
				},
			},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		// Old invitation for other-admin → hasOldAdmin=true and TIP message at end.
		writeJSON(w, []map[string]interface{}{
			{
				"id":         2,
				"invitee":    map[string]string{"login": "other-admin"},
				"created_at": "2020-01-01T00:00:00Z",
			},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators/newuser", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]interface{}{"id": 1})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := addUserAsRepoCollaborator(ctx, client, "mdsol", "my-repo", "newuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// findAndReportTeamsWithAccessToAllRepos – close matches + exact matches
// ---------------------------------------------------------------------------

func TestFindAndReportTeamsWithAccessToAllRepos_WithCloseMatch(t *testing.T) {
	ctx := context.Background()
	// r1+r2: Team A + Team B; r3: only Team A.
	// Team A → exact (3/3), Team B → close (2/3 = 67%).
	bothTeams := []map[string]interface{}{
		{"name": "Team A", "slug": "team-a", "html_url": "https://github.com", "permission": "push"},
		{"name": "Team B", "slug": "team-b", "html_url": "https://github.com", "permission": "pull"},
	}
	onlyA := []map[string]interface{}{
		{"name": "Team A", "slug": "team-a", "html_url": "https://github.com", "permission": "push"},
	}
	configs := []struct {
		repo  string
		teams []map[string]interface{}
	}{
		{"r1", bothTeams}, {"r2", bothTeams}, {"r3", onlyA},
	}
	mux := http.NewServeMux()
	for _, c := range configs {
		c := c
		mux.HandleFunc("/repos/mdsol/"+c.repo, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{"name": c.repo, "owner": map[string]string{"login": "mdsol"}})
		})
		mux.HandleFunc("/repos/mdsol/"+c.repo+"/teams", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, c.teams)
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	findAndReportTeamsWithAccessToAllRepos(ctx, client, "mdsol", []string{"r1", "r2", "r3"})
}

func TestFindAndReportTeamsWithAccessToAllRepos_AnalysisError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	for _, repo := range []string{"x1", "x2"} {
		repo := repo
		mux.HandleFunc("/repos/mdsol/"+repo, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	findAndReportTeamsWithAccessToAllRepos(ctx, client, "mdsol", []string{"x1", "x2"})
}

// ---------------------------------------------------------------------------
// findTeamsWithAccessToAllRepos – error bubble-up path
// ---------------------------------------------------------------------------

func TestFindTeamsWithAccessToAllRepos_Error(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	// Both repos return 404 → analysis errors → wrapper returns nil, error.
	for _, repo := range []string{"fail1", "fail2"} {
		repo := repo
		mux.HandleFunc("/repos/mdsol/"+repo, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	teams, err := findTeamsWithAccessToAllRepos(ctx, client, "mdsol", []string{"fail1", "fail2"})
	if err == nil {
		t.Error("expected error when all repos fail, got nil")
	}
	if teams != nil {
		t.Errorf("expected nil teams on error, got %v", teams)
	}
}

// ---------------------------------------------------------------------------
// listRepositoryCollaborators – added-time from MemberEvent
// ---------------------------------------------------------------------------

func TestListRepositoryCollaborators_WithMemberEvent(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"login":    "user1",
				"html_url": "https://github.com/user1",
				"permissions": map[string]bool{
					"admin": false, "maintain": false,
					"push": true, "triage": false, "pull": true,
				},
			},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{}) // no invitations for user1
	})
	mux.HandleFunc("/repos/mdsol/my-repo/events", func(w http.ResponseWriter, r *http.Request) {
		// Return a MemberEvent for user1 → eventMap gets populated.
		writeJSON(w, []map[string]interface{}{
			{
				"type":       "MemberEvent",
				"created_at": "2023-06-01T12:00:00Z",
				"payload": map[string]interface{}{
					"action": "added",
					"member": map[string]string{"login": "user1"},
				},
			},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators/user1/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"permission": "write",
			"user":       map[string]string{"login": "user1"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := listRepositoryCollaborators(ctx, client, "mdsol", "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// addUserAsRepoCollaborator – target user has recent admin invitation (< 24h)
// ---------------------------------------------------------------------------

func TestAddUserAsRepoCollaborator_TargetUserRecentAdmin(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"login": "newuser",
				"permissions": map[string]bool{
					"admin": true, "push": true, "pull": true,
				},
			},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/collaborators/newuser/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"permission": "admin",
			"user":       map[string]string{"login": "newuser"},
		})
	})
	mux.HandleFunc("/repos/mdsol/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		// Return a RECENT invitation for newuser → triggers the < 24h early-return path.
		recentTimestamp := time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339)
		writeJSON(w, []map[string]interface{}{
			{
				"id":         1,
				"invitee":    map[string]string{"login": "newuser"},
				"created_at": recentTimestamp,
			},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := addUserAsRepoCollaborator(ctx, client, "mdsol", "my-repo", "newuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
