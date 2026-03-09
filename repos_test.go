package main

import (
	"context"
	"net/http"
	"sort"
	"testing"
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
