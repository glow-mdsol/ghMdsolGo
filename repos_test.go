package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v84/github"
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
// actions cache usage
// ---------------------------------------------------------------------------

func TestFormatByteSize(t *testing.T) {
	testCases := []struct {
		name string
		size int64
		want string
	}{
		{name: "bytes", size: 512, want: "512 B"},
		{name: "kibibytes", size: 2048, want: "2.0 KiB"},
		{name: "mebibytes", size: 5 * 1024 * 1024, want: "5.00 MiB"},
		{name: "gibibytes", size: 3 * 1024 * 1024 * 1024, want: "3.00 GiB"},
	}

	for _, tc := range testCases {
		if got := formatByteSize(tc.size); got != tc.want {
			t.Errorf("%s: formatByteSize(%d) = %q, want %q", tc.name, tc.size, got, tc.want)
		}
	}
}

func TestTopActionsCacheUsage_SortsAndLimits(t *testing.T) {
	usages := []*github.ActionsCacheUsage{
		{FullName: "example-org/repo-b", ActiveCachesSizeInBytes: 20, ActiveCachesCount: 1},
		{FullName: "example-org/repo-c", ActiveCachesSizeInBytes: 20, ActiveCachesCount: 2},
		{FullName: "example-org/repo-a", ActiveCachesSizeInBytes: 30, ActiveCachesCount: 3},
	}

	top := topActionsCacheUsage(usages, 2)
	if len(top) != 2 {
		t.Fatalf("len(top) = %d, want 2", len(top))
	}
	if top[0].FullName != "example-org/repo-a" {
		t.Errorf("top[0] = %q, want %q", top[0].FullName, "example-org/repo-a")
	}
	if top[1].FullName != "example-org/repo-b" {
		t.Errorf("top[1] = %q, want %q", top[1].FullName, "example-org/repo-b")
	}

	if usages[0].FullName != "example-org/repo-b" {
		t.Error("topActionsCacheUsage should not mutate the input slice order")
	}
}

func TestFormatActionsCacheUsageReport_NoUsage(t *testing.T) {
	report := formatActionsCacheUsageReport("example-org", nil, nil, 10)
	if !strings.Contains(report, "No repositories are currently using GitHub Actions cache storage in example-org.") {
		t.Errorf("unexpected report: %q", report)
	}
}

func TestFormatActionsCacheUsageReport_IncludesTopRepos(t *testing.T) {
	usages := []*github.ActionsCacheUsage{
		{FullName: "example-org/repo-a", ActiveCachesSizeInBytes: 3 * 1024 * 1024 * 1024, ActiveCachesCount: 4},
		{FullName: "example-org/repo-b", ActiveCachesSizeInBytes: 512, ActiveCachesCount: 1},
	}

	report := formatActionsCacheUsageReport("example-org", usages, nil, 10)
	if !strings.Contains(report, "Top 2 repositories by GitHub Actions cache storage in example-org:") {
		t.Errorf("missing header in report: %q", report)
	}
	if !strings.Contains(report, "1. example-org/repo-a") {
		t.Errorf("missing top repo in report: %q", report)
	}
	if !strings.Contains(report, "3.00 GiB") {
		t.Errorf("missing formatted size in report: %q", report)
	}
	if !strings.Contains(report, "4 active caches") {
		t.Errorf("missing cache count in report: %q", report)
	}
}

func TestFormatActionsCacheUsageReport_IncludesBillingSummary(t *testing.T) {
	usages := []*github.ActionsCacheUsage{
		{FullName: "example-org/repo-a", ActiveCachesSizeInBytes: 1024, ActiveCachesCount: 1},
	}
	billing := &github.StorageBilling{
		EstimatedPaidStorageForMonth: 42,
		EstimatedStorageForMonth:     64,
		DaysLeftInBillingCycle:       12,
	}

	report := formatActionsCacheUsageReport("example-org", usages, billing, 10)
	if !strings.Contains(report, "Org Actions shared storage billing summary:") {
		t.Errorf("missing billing header in report: %q", report)
	}
	if !strings.Contains(report, "Billable constrained storage: 42 GB-month") {
		t.Errorf("missing billing summary in report: %q", report)
	}
	if !strings.Contains(report, "Estimated total shared storage: 64 GB-month") {
		t.Errorf("missing total storage summary in report: %q", report)
	}
	if !strings.Contains(report, "Days left in billing cycle: 12") {
		t.Errorf("missing billing cycle summary in report: %q", report)
	}
	if !strings.Contains(report, "1. example-org/repo-a") {
		t.Errorf("missing repo usage in report: %q", report)
	}
}

func TestGetActionsCacheUsageByRepoForOrg(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/example-org/actions/cache/usage-by-repository", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("per_page"); got != "100" {
			t.Errorf("per_page = %q, want 100", got)
		}
		writeJSON(w, map[string]interface{}{
			"total_count": 2,
			"repository_cache_usages": []map[string]interface{}{
				{"full_name": "example-org/repo-a", "active_caches_size_in_bytes": 1000, "active_caches_count": 2},
				{"full_name": "example-org/repo-b", "active_caches_size_in_bytes": 500, "active_caches_count": 1},
			},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	usages, err := getActionsCacheUsageByRepoForOrg(ctx, client, "example-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want 2", len(usages))
	}
	if usages[0].FullName != "example-org/repo-a" {
		t.Errorf("usages[0].FullName = %q, want %q", usages[0].FullName, "example-org/repo-a")
	}
	if usages[1].ActiveCachesCount != 1 {
		t.Errorf("usages[1].ActiveCachesCount = %d, want 1", usages[1].ActiveCachesCount)
	}
}

func TestGetActionsStorageBillingForOrg(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/example-org/settings/billing/shared-storage", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"days_left_in_billing_cycle":       20,
			"estimated_paid_storage_for_month": 15,
			"estimated_storage_for_month":      25,
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	billing, err := getActionsStorageBillingForOrg(ctx, client, "example-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if billing.EstimatedPaidStorageForMonth != 15 {
		t.Errorf("billing.EstimatedPaidStorageForMonth = %d, want 15", billing.EstimatedPaidStorageForMonth)
	}
	if billing.EstimatedStorageForMonth != 25 {
		t.Errorf("billing.EstimatedStorageForMonth = %d, want 25", billing.EstimatedStorageForMonth)
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

	if isRepository(ctx, client, "example-org", "user@example.com") {
		t.Error("isRepository with email slug should return false")
	}
}

func TestIsRepository_Found(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/example-org", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"login": "example-org"})
	})
	mux.HandleFunc("/repos/example-org/my-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "my-repo", "owner": map[string]string{"login": "example-org"}})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	if !isRepository(ctx, client, "example-org", "my-repo") {
		t.Error("isRepository expected true for existing repo")
	}
}

func TestIsRepository_NotFound(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/example-org", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"login": "example-org"})
	})
	mux.HandleFunc("/repos/example-org/missing-repo", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	if isRepository(ctx, client, "example-org", "missing-repo") {
		t.Error("isRepository expected false for missing repo")
	}
}

func TestIsRepository_OrgNotFound(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/example-org", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	if isRepository(ctx, client, "example-org", "some-repo") {
		t.Error("isRepository expected false when org not found")
	}
}

// ---------------------------------------------------------------------------
// checkRepository
// ---------------------------------------------------------------------------

func TestCheckRepository_Exists(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/example-org/my-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"name":        "my-repo",
			"description": "A test repo",
			"owner":       map[string]string{"login": "example-org"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	info, err := checkRepository(ctx, client, "example-org", "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.name != "my-repo" {
		t.Errorf("name = %q, want %q", info.name, "my-repo")
	}
	if info.owner != "example-org" {
		t.Errorf("owner = %q, want %q", info.owner, "example-org")
	}
	if info.description != "A test repo" {
		t.Errorf("description = %q, want %q", info.description, "A test repo")
	}
}

func TestCheckRepository_NoDescription(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/example-org/bare-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"name":  "bare-repo",
			"owner": map[string]string{"login": "example-org"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	info, err := checkRepository(ctx, client, "example-org", "bare-repo")
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
	mux.HandleFunc("/repos/example-org/missing", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	_, err := checkRepository(ctx, client, "example-org", "missing")
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
	mux.HandleFunc("/repos/example-org/my-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "my-repo", "owner": map[string]string{"login": "example-org"}})
	})
	mux.HandleFunc("/repos/example-org/my-repo/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"name":        "Team Alpha",
				"slug":        "team-alpha",
				"html_url":    "https://github.com/orgs/example-org/teams/team-alpha",
				"permission":  "pull",
				"description": "Alpha team",
			},
			{
				"name":       "Team Beta",
				"slug":       "team-beta",
				"html_url":   "https://github.com/orgs/example-org/teams/team-beta",
				"permission": "push",
			},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	teams, err := getRepositoryTeams(ctx, client, "example-org", "my-repo")
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
	mux.HandleFunc("/repos/example-org/empty-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "empty-repo", "owner": map[string]string{"login": "example-org"}})
	})
	mux.HandleFunc("/repos/example-org/empty-repo/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	teams, err := getRepositoryTeams(ctx, client, "example-org", "empty-repo")
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
	mux.HandleFunc("/repos/example-org/missing", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	_, err := getRepositoryTeams(ctx, client, "example-org", "missing")
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

	_, err := findTeamsWithAccessAnalysis(ctx, client, "example-org", nil)
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
		mux.HandleFunc("/repos/example-org/"+repo, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{"name": repo, "owner": map[string]string{"login": "example-org"}})
		})
		mux.HandleFunc("/repos/example-org/"+repo+"/teams", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []map[string]interface{}{
				{"name": "Team Alpha", "slug": "team-alpha", "html_url": "https://github.com", "permission": "push"},
				{"name": "Team Beta", "slug": "team-beta", "html_url": "https://github.com", "permission": "pull"},
			})
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	result, err := findTeamsWithAccessAnalysis(ctx, client, "example-org", []string{"repo1", "repo2"})
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

	mux.HandleFunc("/repos/example-org/repo1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "repo1", "owner": map[string]string{"login": "example-org"}})
	})
	mux.HandleFunc("/repos/example-org/repo1/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "Team Alpha", "slug": "team-alpha", "html_url": "https://github.com", "permission": "push"},
			{"name": "Team Beta", "slug": "team-beta", "html_url": "https://github.com", "permission": "pull"},
		})
	})
	mux.HandleFunc("/repos/example-org/repo2", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "repo2", "owner": map[string]string{"login": "example-org"}})
	})
	mux.HandleFunc("/repos/example-org/repo2/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "Team Alpha", "slug": "team-alpha", "html_url": "https://github.com", "permission": "push"},
			{"name": "Team Beta", "slug": "team-beta", "html_url": "https://github.com", "permission": "pull"},
		})
	})
	mux.HandleFunc("/repos/example-org/repo3", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "repo3", "owner": map[string]string{"login": "example-org"}})
	})
	mux.HandleFunc("/repos/example-org/repo3/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "Team Beta", "slug": "team-beta", "html_url": "https://github.com", "permission": "pull"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	result, err := findTeamsWithAccessAnalysis(ctx, client, "example-org", []string{"repo1", "repo2", "repo3"})
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

	mux.HandleFunc("/repos/example-org/repo1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "repo1", "owner": map[string]string{"login": "example-org"}})
	})
	mux.HandleFunc("/repos/example-org/repo1/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "Team A", "slug": "team-a", "html_url": "https://github.com", "permission": "push"},
		})
	})
	mux.HandleFunc("/repos/example-org/repo2", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "repo2", "owner": map[string]string{"login": "example-org"}})
	})
	mux.HandleFunc("/repos/example-org/repo2/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "Team B", "slug": "team-b", "html_url": "https://github.com", "permission": "pull"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	result, err := findTeamsWithAccessAnalysis(ctx, client, "example-org", []string{"repo1", "repo2"})
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
		mux.HandleFunc("/repos/example-org/"+repo, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{"name": repo, "owner": map[string]string{"login": "example-org"}})
		})
		mux.HandleFunc("/repos/example-org/"+repo+"/teams", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []map[string]interface{}{
				{"name": "Team X", "slug": "team-x", "html_url": "https://github.com", "permission": "push"},
			})
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	teams, err := findTeamsWithAccessToAllRepos(ctx, client, "example-org", []string{"r1", "r2"})
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
	mux.HandleFunc("/repos/example-org/existing-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "existing-repo", "owner": map[string]string{"login": "example-org"}})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	info := repositoryInfo{owner: "example-org", name: "existing-repo", description: "test"}
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
	mux.HandleFunc("/repos/example-org/some-repo", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	info := repositoryInfo{owner: "example-org", name: "some-repo"}
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
	mux.HandleFunc("/repos/example-org/my-repo/vulnerability-alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNoContent) // 204 → enabled=true
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	enabled, err := enableVulnerabilityAlerts(ctx, client, "example-org", "my-repo")
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
	mux.HandleFunc("/repos/example-org/my-repo/vulnerability-alerts", func(w http.ResponseWriter, r *http.Request) {
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

	enabled, err := enableVulnerabilityAlerts(ctx, client, "example-org", "my-repo")
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
	mux.HandleFunc("/repos/example-org/missing-repo/vulnerability-alerts", func(w http.ResponseWriter, r *http.Request) {
		// 500 causes a genuine GET-level error (not the 404 "not enabled" path)
		http.Error(w, `{"message":"Internal Server Error"}`, http.StatusInternalServerError)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	_, err := enableVulnerabilityAlerts(ctx, client, "example-org", "missing-repo")
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
	mux.HandleFunc("/repos/example-org/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := listRepositoryCollaborators(ctx, client, "example-org", "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListRepositoryCollaborators_WithCollaborator(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/example-org/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("/repos/example-org/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/example-org/my-repo/events", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/example-org/my-repo/collaborators/user1/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"permission": "write",
			"user":       map[string]string{"login": "user1"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := listRepositoryCollaborators(ctx, client, "example-org", "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListRepositoryCollaborators_APIError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/example-org/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := listRepositoryCollaborators(ctx, client, "example-org", "my-repo")
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
	mux.HandleFunc("/repos/example-org/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{}) // no existing collaborators
	})
	mux.HandleFunc("/repos/example-org/my-repo/collaborators/newuser", func(w http.ResponseWriter, r *http.Request) {
		// PUT adds the collaborator → 201 invitation created
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]interface{}{"id": 1})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := addUserAsRepoCollaborator(ctx, client, "example-org", "my-repo", "newuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddUserAsRepoCollaborator_AlreadyHasAccess(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/example-org/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/example-org/my-repo/collaborators/existinguser", func(w http.ResponseWriter, r *http.Request) {
		// PUT → 204 means user already had access
		w.WriteHeader(http.StatusNoContent)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := addUserAsRepoCollaborator(ctx, client, "example-org", "my-repo", "existinguser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddUserAsRepoCollaborator_OtherAdminUnknownDate(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/example-org/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("/repos/example-org/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/example-org/my-repo/events", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/example-org/my-repo/collaborators/newuser", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]interface{}{"id": 1})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := addUserAsRepoCollaborator(ctx, client, "example-org", "my-repo", "newuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddUserAsRepoCollaborator_APIError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/example-org/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := addUserAsRepoCollaborator(ctx, client, "example-org", "my-repo", "newuser")
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
		mux.HandleFunc("/repos/example-org/"+repo, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{"name": repo, "owner": map[string]string{"login": "example-org"}})
		})
		mux.HandleFunc("/repos/example-org/"+repo+"/teams", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []map[string]interface{}{
				{"name": "All Access Team", "slug": "all-access-team", "html_url": "https://github.com", "permission": "push"},
			})
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	// Must not panic/error; side effects are only stdout logging.
	findAndReportTeamsWithAccessToAllRepos(ctx, client, "example-org", []string{"r1", "r2"})
}

func TestFindAndReportTeamsWithAccessToAllRepos_NoMatch(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	for i, repo := range []string{"r1", "r2"} {
		repo := repo
		i := i
		slug := fmt.Sprintf("team-%d", i)
		mux.HandleFunc("/repos/example-org/"+repo, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{"name": repo, "owner": map[string]string{"login": "example-org"}})
		})
		mux.HandleFunc("/repos/example-org/"+repo+"/teams", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []map[string]interface{}{
				{"name": "Team " + slug, "slug": slug, "html_url": "https://github.com", "permission": "push"},
			})
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	findAndReportTeamsWithAccessToAllRepos(ctx, client, "example-org", []string{"r1", "r2"})
}

// ---------------------------------------------------------------------------
// listRepositoryCollaborators – invitation and admin paths
// ---------------------------------------------------------------------------

func TestListRepositoryCollaborators_WithInvitation(t *testing.T) {
	// Collaborator found via invitation (added time from invitation source).
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/example-org/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("/repos/example-org/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"id":         1,
				"invitee":    map[string]string{"login": "user1"},
				"created_at": "2023-01-01T00:00:00Z",
			},
		})
	})
	mux.HandleFunc("/repos/example-org/my-repo/events", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/example-org/my-repo/collaborators/user1/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"permission": "write",
			"user":       map[string]string{"login": "user1"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := listRepositoryCollaborators(ctx, client, "example-org", "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListRepositoryCollaborators_AdminOldInvitation(t *testing.T) {
	// Admin user with a very old invitation → "Admin access granted >24 hours ago" warning.
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/example-org/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("/repos/example-org/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"id":         1,
				"invitee":    map[string]string{"login": "admin1"},
				"created_at": "2020-01-01T00:00:00Z", // very old → >24h warning
			},
		})
	})
	mux.HandleFunc("/repos/example-org/my-repo/events", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	mux.HandleFunc("/repos/example-org/my-repo/collaborators/admin1/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"permission": "admin",
			"user":       map[string]string{"login": "admin1"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := listRepositoryCollaborators(ctx, client, "example-org", "my-repo")
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
	mux.HandleFunc("/repos/example-org/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("/repos/example-org/my-repo/collaborators/newuser/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"permission": "admin",
			"user":       map[string]string{"login": "newuser"},
		})
	})
	mux.HandleFunc("/repos/example-org/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := addUserAsRepoCollaborator(ctx, client, "example-org", "my-repo", "newuser")
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
	mux.HandleFunc("/repos/example-org/error-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "error-repo", "owner": map[string]string{"login": "example-org"}})
	})
	mux.HandleFunc("/repos/example-org/error-repo/teams", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	_, err := getRepositoryTeams(ctx, client, "example-org", "error-repo")
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
	mux.HandleFunc("/repos/example-org/bad1", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/repos/example-org/bad2", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	_, err := findTeamsWithAccessAnalysis(ctx, client, "example-org", []string{"bad1", "bad2"})
	if err == nil {
		t.Error("expected error when all repos fail, got nil")
	}
}

func TestFindTeamsWithAccessAnalysis_PartialRepoError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/example-org/good-repo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "good-repo", "owner": map[string]string{"login": "example-org"}})
	})
	mux.HandleFunc("/repos/example-org/good-repo/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "Team A", "slug": "team-a", "html_url": "https://github.com", "permission": "push"},
		})
	})
	mux.HandleFunc("/repos/example-org/bad-repo", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	result, err := findTeamsWithAccessAnalysis(ctx, client, "example-org", []string{"good-repo", "bad-repo"})
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
	mux.HandleFunc("/repos/example-org/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"login": "other-admin",
				"permissions": map[string]bool{
					"admin": true, "push": true, "pull": true,
				},
			},
		})
	})
	mux.HandleFunc("/repos/example-org/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		// Old invitation for other-admin → hasOldAdmin=true and TIP message at end.
		writeJSON(w, []map[string]interface{}{
			{
				"id":         2,
				"invitee":    map[string]string{"login": "other-admin"},
				"created_at": "2020-01-01T00:00:00Z",
			},
		})
	})
	mux.HandleFunc("/repos/example-org/my-repo/collaborators/newuser", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]interface{}{"id": 1})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := addUserAsRepoCollaborator(ctx, client, "example-org", "my-repo", "newuser")
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
		mux.HandleFunc("/repos/example-org/"+c.repo, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{"name": c.repo, "owner": map[string]string{"login": "example-org"}})
		})
		mux.HandleFunc("/repos/example-org/"+c.repo+"/teams", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, c.teams)
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	findAndReportTeamsWithAccessToAllRepos(ctx, client, "example-org", []string{"r1", "r2", "r3"})
}

func TestFindAndReportTeamsWithAccessToAllRepos_AnalysisError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	for _, repo := range []string{"x1", "x2"} {
		repo := repo
		mux.HandleFunc("/repos/example-org/"+repo, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	findAndReportTeamsWithAccessToAllRepos(ctx, client, "example-org", []string{"x1", "x2"})
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
		mux.HandleFunc("/repos/example-org/"+repo, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		})
	}
	client, teardown := newTestClient(mux)
	defer teardown()

	teams, err := findTeamsWithAccessToAllRepos(ctx, client, "example-org", []string{"fail1", "fail2"})
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
	mux.HandleFunc("/repos/example-org/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("/repos/example-org/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []interface{}{}) // no invitations for user1
	})
	mux.HandleFunc("/repos/example-org/my-repo/events", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("/repos/example-org/my-repo/collaborators/user1/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"permission": "write",
			"user":       map[string]string{"login": "user1"},
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	err := listRepositoryCollaborators(ctx, client, "example-org", "my-repo")
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
	mux.HandleFunc("/repos/example-org/my-repo/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{
				"login": "newuser",
				"permissions": map[string]bool{
					"admin": true, "push": true, "pull": true,
				},
			},
		})
	})
	mux.HandleFunc("/repos/example-org/my-repo/collaborators/newuser/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"permission": "admin",
			"user":       map[string]string{"login": "newuser"},
		})
	})
	mux.HandleFunc("/repos/example-org/my-repo/invitations", func(w http.ResponseWriter, r *http.Request) {
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

	err := addUserAsRepoCollaborator(ctx, client, "example-org", "my-repo", "newuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// reportRecentAdminGrants
// ---------------------------------------------------------------------------

func TestReportRecentAdminGrants_NoResults(t *testing.T) {
	got := reportRecentAdminGrants("example-org", nil)
	if !strings.Contains(got, "No admin access grants detected") {
		t.Errorf("expected no-results message, got %q", got)
	}
}

func TestReportRecentAdminGrants_WithResults(t *testing.T) {
	grantedAt := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	results := []adminGrantResult{
		{
			repo:      "sample-orchestrator",
			login:     "alice",
			actor:     "org-admin",
			grantedAt: grantedAt,
			accessURL: "https://github.com/example-org/sample-orchestrator/settings/access",
		},
	}
	got := reportRecentAdminGrants("example-org", results)
	if !strings.Contains(got, "alice") {
		t.Errorf("expected login 'alice' in output, got %q", got)
	}
	if !strings.Contains(got, "sample-orchestrator") {
		t.Errorf("expected repo name in output, got %q", got)
	}
	if !strings.Contains(got, "https://github.com/example-org/sample-orchestrator/settings/access") {
		t.Errorf("expected access URL in output, got %q", got)
	}
	if !strings.Contains(got, "Granted by: org-admin") {
		t.Errorf("expected actor in output, got %q", got)
	}
	if !strings.Contains(got, "2026-04-15 10:00:00 UTC") {
		t.Errorf("expected formatted grant time in output, got %q", got)
	}
}

func TestReportRecentAdminGrants_MultipleResults(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	results := []adminGrantResult{
		{repo: "repo-a", login: "alice", actor: "admin-a", grantedAt: now, accessURL: "https://github.com/example-org/repo-a/settings/access"},
		{repo: "repo-a", login: "bob", actor: "admin-b", grantedAt: now, accessURL: "https://github.com/example-org/repo-a/settings/access"},
		{repo: "repo-b", login: "carol", actor: "admin-c", grantedAt: now, accessURL: "https://github.com/example-org/repo-b/settings/access"},
	}
	got := reportRecentAdminGrants("example-org", results)
	if !strings.Contains(got, "alice") || !strings.Contains(got, "bob") || !strings.Contains(got, "carol") {
		t.Errorf("expected all logins in output, got %q", got)
	}
	if !strings.Contains(got, "Granted by: admin-a") || !strings.Contains(got, "Granted by: admin-c") {
		t.Errorf("expected actor lines in output, got %q", got)
	}
	if !strings.Contains(got, "repo-b/settings/access") {
		t.Errorf("expected repo-b access URL in output, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// getOrgRepos
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// findRecentAdminGrants (audit log approach)
// ---------------------------------------------------------------------------

// auditLogEntry builds a JSON map that mimics a GitHub audit log response for
// repo.add_member events. "repo" and "permission" land in AdditionalFields.
func auditLogEntry(login, repo, permission string, createdAtMs int64) map[string]interface{} {
	return map[string]interface{}{
		"action":     "repo.add_member",
		"actor":      "an-admin",
		"user":       login,
		"repo":       repo,
		"permission": permission,
		"created_at": createdAtMs,
	}
}

func TestFindRecentAdminGrants_AdminGranted(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()

	recentMs := time.Now().Add(-2 * time.Hour).UnixMilli()
	mux.HandleFunc("/orgs/example-org/audit-log", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			auditLogEntry("alice", "example-org/my-repo", "admin", recentMs),
		})
	})
	mux.HandleFunc("/repos/example-org/my-repo/collaborators/alice/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"permission": "admin", "user": map[string]string{"login": "alice"}})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	results, err := findRecentAdminGrants(ctx, client, "example-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].login != "alice" {
		t.Errorf("login = %q, want alice", results[0].login)
	}
	if results[0].repo != "my-repo" {
		t.Errorf("repo = %q, want my-repo", results[0].repo)
	}
	if results[0].actor != "an-admin" {
		t.Errorf("actor = %q, want an-admin", results[0].actor)
	}
	wantURL := "https://github.com/example-org/my-repo/settings/access"
	if results[0].accessURL != wantURL {
		t.Errorf("accessURL = %q, want %q", results[0].accessURL, wantURL)
	}
}

func TestFindRecentAdminGrants_NonAdminSkipped(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()

	recentMs := time.Now().Add(-1 * time.Hour).UnixMilli()
	mux.HandleFunc("/orgs/example-org/audit-log", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			auditLogEntry("carol", "example-org/my-repo", "write", recentMs),
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	results, err := findRecentAdminGrants(ctx, client, "example-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-admin grant, got %d", len(results))
	}
}

func TestFindRecentAdminGrants_EntryTooOld(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()

	oldMs := time.Now().Add(-48 * time.Hour).UnixMilli()
	mux.HandleFunc("/orgs/example-org/audit-log", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			auditLogEntry("bob", "example-org/my-repo", "admin", oldMs),
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	results, err := findRecentAdminGrants(ctx, client, "example-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for old entry, got %d", len(results))
	}
}

func TestFindRecentAdminGrants_RevokedAccessExcluded(t *testing.T) {
	// Audit log shows admin grant, but GetPermissionLevel says write → excluded.
	ctx := context.Background()
	mux := http.NewServeMux()

	recentMs := time.Now().Add(-1 * time.Hour).UnixMilli()
	mux.HandleFunc("/orgs/example-org/audit-log", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			auditLogEntry("dave", "example-org/a-repo", "admin", recentMs),
		})
	})
	mux.HandleFunc("/repos/example-org/a-repo/collaborators/dave/permission", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"permission": "write", "user": map[string]string{"login": "dave"}})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	results, err := findRecentAdminGrants(ctx, client, "example-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results when access revoked, got %d", len(results))
	}
}

func TestFindRecentAdminGrants_DuplicateEntryDeduped(t *testing.T) {
	// Two audit log entries for the same (repo, user) → only one permission check.
	ctx := context.Background()
	mux := http.NewServeMux()

	recentMs := time.Now().Add(-1 * time.Hour).UnixMilli()
	callCount := 0
	mux.HandleFunc("/orgs/example-org/audit-log", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			auditLogEntry("eve", "example-org/dup-repo", "admin", recentMs),
			auditLogEntry("eve", "example-org/dup-repo", "admin", recentMs),
		})
	})
	mux.HandleFunc("/repos/example-org/dup-repo/collaborators/eve/permission", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		writeJSON(w, map[string]interface{}{"permission": "admin", "user": map[string]string{"login": "eve"}})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	results, err := findRecentAdminGrants(ctx, client, "example-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (deduped), got %d", len(results))
	}
	if callCount != 1 {
		t.Errorf("GetPermissionLevel called %d times, want 1", callCount)
	}
}

func TestFindRecentAdminGrants_WrongOrgSkipped(t *testing.T) {
	// Audit log entry for a different org's repo should be skipped.
	ctx := context.Background()
	mux := http.NewServeMux()

	recentMs := time.Now().Add(-1 * time.Hour).UnixMilli()
	mux.HandleFunc("/orgs/example-org/audit-log", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			auditLogEntry("frank", "other-org/some-repo", "admin", recentMs),
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	results, err := findRecentAdminGrants(ctx, client, "example-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for wrong-org repo, got %d", len(results))
	}
}

func TestFindRecentAdminGrants_APIError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/example-org/audit-log", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	_, err := findRecentAdminGrants(ctx, client, "example-org")
	if err == nil {
		t.Error("expected error when audit log API returns 403, got nil")
	}
}
