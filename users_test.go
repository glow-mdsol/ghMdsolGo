package main

import (
	"context"
	"net/http"
	"testing"
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
