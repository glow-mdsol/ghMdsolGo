package main

import "testing"

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
