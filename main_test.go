package main

import (
	"context"
	"net/http"
	"testing"
)

// ---------------------------------------------------------------------------
// contains
// ---------------------------------------------------------------------------

func TestContains_True(t *testing.T) {
	s := []string{"mdsol.com", "shyftanalytics.com", "3ds.com"}
	for _, v := range s {
		if !contains(s, v) {
			t.Errorf("contains(%q) expected true", v)
		}
	}
}

func TestContains_False(t *testing.T) {
	s := []string{"mdsol.com", "shyftanalytics.com", "3ds.com"}
	if contains(s, "github.com") {
		t.Error("contains(github.com) expected false")
	}
}

func TestContains_EmptySlice(t *testing.T) {
	if contains([]string{}, "anything") {
		t.Error("contains on empty slice should return false")
	}
}

func TestContains_EmptyString(t *testing.T) {
	s := []string{"a", "", "b"}
	if !contains(s, "") {
		t.Error("contains(\"\") expected true when empty string is in slice")
	}
}

// ---------------------------------------------------------------------------
// detectEntityType
// ---------------------------------------------------------------------------

func setupDetectEntityMux(repoSlug, userSlug, memberSlug string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"login": "mdsol"})
	})
	if repoSlug != "" {
		mux.HandleFunc("/repos/mdsol/"+repoSlug, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{"name": repoSlug, "owner": map[string]string{"login": "mdsol"}})
		})
	}
	if userSlug != "" {
		mux.HandleFunc("/users/"+userSlug, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]string{"login": userSlug})
		})
	}
	if memberSlug != "" {
		mux.HandleFunc("/orgs/mdsol/memberships/"+memberSlug, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{
				"state":        "active",
				"role":         "member",
				"user":         map[string]string{"login": memberSlug},
				"organization": map[string]string{"login": "mdsol"},
				"url":          "https://api.github.com/orgs/mdsol/memberships/" + memberSlug,
			})
		})
	}
	return mux
}

func TestDetectEntityType_IsRepository(t *testing.T) {
	ctx := context.Background()
	mux := setupDetectEntityMux("my-repo", "", "")
	client, teardown := newTestClient(mux)
	defer teardown()

	entType, name := detectEntityType(ctx, client, nil, "my-repo")
	if entType != entityRepository {
		t.Errorf("entityType = %v, want entityRepository", entType)
	}
	if name != "my-repo" {
		t.Errorf("name = %q, want %q", name, "my-repo")
	}
}

func TestDetectEntityType_IsUserAndMember(t *testing.T) {
	ctx := context.Background()
	// No repo match for this slug, but user exists and is a member.
	mux := setupDetectEntityMux("", "someuser", "someuser")
	// Repo check: org exists but repo 404s.
	mux.HandleFunc("/repos/mdsol/someuser", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	entType, name := detectEntityType(ctx, client, nil, "someuser")
	if entType != entityUser {
		t.Errorf("entityType = %v, want entityUser", entType)
	}
	if name != "someuser" {
		t.Errorf("name = %q, want %q", name, "someuser")
	}
}

func TestDetectEntityType_IsUserNotMember(t *testing.T) {
	ctx := context.Background()
	mux := setupDetectEntityMux("", "outsider", "")
	mux.HandleFunc("/repos/mdsol/outsider", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	// Membership check returns 404 → not a member.
	mux.HandleFunc("/orgs/mdsol/memberships/outsider", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	entType, _ := detectEntityType(ctx, client, nil, "outsider")
	if entType != entityUnknown {
		t.Errorf("entityType = %v, want entityUnknown", entType)
	}
}

func TestDetectEntityType_Unknown(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/mdsol", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"login": "mdsol"})
	})
	mux.HandleFunc("/repos/mdsol/nobody", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/users/nobody", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	client, teardown := newTestClient(mux)
	defer teardown()

	entType, _ := detectEntityType(ctx, client, nil, "nobody")
	if entType != entityUnknown {
		t.Errorf("entityType = %v, want entityUnknown", entType)
	}
}
