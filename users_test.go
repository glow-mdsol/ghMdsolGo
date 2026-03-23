package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-github/v84/github"
)

// ---------------------------------------------------------------------------
// slugify
// ---------------------------------------------------------------------------

func TestSlugify(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Team Medidata", "team-medidata"},
		{"My Team Name", "my-team-name"},
		{"lowercase", "lowercase"},
		{"UPPERCASE", "uppercase"},
		{"Mixed Case Team", "mixed-case-team"},
		{"already-slugged", "already-slugged"},
		{"", ""},
	}
	for _, tc := range cases {
		got := slugify(tc.input)
		if got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// resolveLogin
// ---------------------------------------------------------------------------

func TestResolveLogin_PlainLogin(t *testing.T) {
	// Non-email slugs are returned unchanged; tc (http.Client) is never called.
	ctx := context.Background()
	login := "someuser"
	got, err := resolveLogin(ctx, nil, &login)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "someuser" {
		t.Errorf("resolveLogin = %q, want %q", got, "someuser")
	}
}

func TestResolveLogin_PreservesCase(t *testing.T) {
	ctx := context.Background()
	login := "CamelCaseUser"
	got, err := resolveLogin(ctx, nil, &login)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "CamelCaseUser" {
		t.Errorf("resolveLogin = %q, want %q", got, "CamelCaseUser")
	}
}

// ---------------------------------------------------------------------------
// isUser
// ---------------------------------------------------------------------------

func TestIsUser_Found(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/users/someuser", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"login": "someuser"})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	login := "someuser"
	if !isUser(ctx, client, &login) {
		t.Error("isUser expected true for existing user")
	}
}

func TestIsUser_NotFound(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/users/ghost", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	login := "ghost"
	if isUser(ctx, client, &login) {
		t.Error("isUser expected false for missing user")
	}
}

// ---------------------------------------------------------------------------
// meetsOrgPrequisites
// ---------------------------------------------------------------------------

func TestMeetsOrgPrequisites_IsMember(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol/memberships/testuser", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"state": "active",
			"role":  "member",
			"user": map[string]string{"login": "testuser"},
			"organization": map[string]string{"login": "mdsol"},
			"url": "https://api.github.com/orgs/mdsol/memberships/testuser",
		})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	login := "testuser"
	u := &github.User{Login: &login}
	ok, code := meetsOrgPrequisites(ctx, client, u)
	if !ok || code != 0 {
		t.Errorf("meetsOrgPrequisites = (%v, %d), want (true, 0)", ok, code)
	}
}

func TestMeetsOrgPrequisites_NotMember(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol/memberships/outsider", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	login := "outsider"
	u := &github.User{Login: &login}
	ok, code := meetsOrgPrequisites(ctx, client, u)
	if ok || code != 1 {
		t.Errorf("meetsOrgPrequisites = (%v, %d), want (false, 1)", ok, code)
	}
}

func TestMeetsOrgPrequisites_OtherError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol/memberships/baduser", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	login := "baduser"
	u := &github.User{Login: &login}
	ok, code := meetsOrgPrequisites(ctx, client, u)
	if ok || code != 2 {
		t.Errorf("meetsOrgPrequisites = (%v, %d), want (false, 2)", ok, code)
	}
}

// ---------------------------------------------------------------------------
// meets2FAPrerequisites
// ---------------------------------------------------------------------------

func TestMeets2FAPrerequisites_Has2FA(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol/members", func(w http.ResponseWriter, r *http.Request) {
		// Return an empty list – testuser is not in the 2FA-disabled list.
		writeJSON(w, []interface{}{})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	login := "testuser"
	u := &github.User{Login: &login}
	ok, code := meets2FAPrerequisites(ctx, client, u)
	if !ok || code != 0 {
		t.Errorf("meets2FAPrerequisites = (%v, %d), want (true, 0)", ok, code)
	}
}

func TestMeets2FAPrerequisites_Missing2FA(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol/members", func(w http.ResponseWriter, r *http.Request) {
		// testuser appears in the 2FA-disabled list.
		writeJSON(w, []map[string]string{{"login": "testuser"}})
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	login := "testuser"
	u := &github.User{Login: &login}
	ok, code := meets2FAPrerequisites(ctx, client, u)
	if ok || code != 4 {
		t.Errorf("meets2FAPrerequisites = (%v, %d), want (false, 4)", ok, code)
	}
}

func TestMeets2FAPrerequisites_APIError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol/members", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	login := "testuser"
	u := &github.User{Login: &login}
	ok, code := meets2FAPrerequisites(ctx, client, u)
	if ok || code != 2 {
		t.Errorf("meets2FAPrerequisites = (%v, %d), want (false, 2)", ok, code)
	}
}
