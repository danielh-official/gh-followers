package main

import (
	"encoding/json"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Two pages, exercising the shape --slurp actually emits. "norepos" has an
// empty repositories.nodes, which is the case that panics if mishandled.
const fixture = `[
 {"data":{"viewer":{"followers":{"nodes":[
   {"login":"small","name":"S","viewerIsFollowing":false,
    "followers":{"totalCount":5},
    "repositories":{"nodes":[{"stargazerCount":900,"nameWithOwner":"small/a"}]}},
   {"login":"norepos","name":null,"viewerIsFollowing":true,
    "followers":{"totalCount":9000},
    "repositories":{"nodes":[]}}
 ]}}}},
 {"data":{"viewer":{"followers":{"nodes":[
   {"login":"big","name":"B","viewerIsFollowing":true,
    "followers":{"totalCount":100},
    "repositories":{"nodes":[{"stargazerCount":5000,"nameWithOwner":"big/z"}]}}
 ]}}}}
]`

func TestFlattenAndSort(t *testing.T) {
	var pages []page
	if err := json.Unmarshal([]byte(fixture), &pages); err != nil {
		t.Fatal(err)
	}

	fs := flatten(pages)
	if len(fs) != 3 {
		t.Fatalf("flatten across pages: got %d followers, want 3", len(fs))
	}

	// A follower with no owned repos must degrade, not panic.
	var norepos follower
	for _, f := range fs {
		if f.Login == "norepos" {
			norepos = f
		}
	}
	if norepos.Stars != 0 || norepos.TopRepo != "-" {
		t.Errorf("empty repositories.nodes: got %d stars / %q, want 0 / \"-\"",
			norepos.Stars, norepos.TopRepo)
	}
	if !norepos.Following {
		t.Error("viewerIsFollowing did not survive decoding")
	}

	sortBy(fs, byStars)
	if got := []string{fs[0].Login, fs[1].Login, fs[2].Login}; got[0] != "big" || got[2] != "norepos" {
		t.Errorf("byStars: got %v, want big, small, norepos", got)
	}

	sortBy(fs, byFollowers)
	if got := []string{fs[0].Login, fs[1].Login, fs[2].Login}; got[0] != "norepos" || got[2] != "small" {
		t.Errorf("byFollowers: got %v, want norepos, big, small", got)
	}
}

// The "s" handler is the one piece of wiring a pty test could not reach:
// it must flip the mode AND rebuild the visible rows, not just the mode.
func TestSortKeyRebuildsRows(t *testing.T) {
	var pages []page
	if err := json.Unmarshal([]byte(fixture), &pages); err != nil {
		t.Fatal(err)
	}

	tm, _ := initialModel().Update(loadedMsg{flatten(pages)})
	m := tm.(model)
	if m.loading {
		t.Fatal("loadedMsg did not clear the loading state")
	}
	if got := m.table.Rows()[0][1]; got != "big" {
		t.Fatalf("default sort: top row is %q, want big (most stars)", got)
	}

	tm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = tm.(model)
	if m.mode != byFollowers {
		t.Errorf("mode after s: got %v, want follower count", m.mode)
	}
	if got := m.table.Rows()[0][1]; got != "norepos" {
		t.Errorf("rows after s: top row is %q, want norepos (most followers)", got)
	}

	tm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = tm.(model)
	if m.mode != byStars || m.table.Rows()[0][1] != "big" {
		t.Error("s did not cycle back to stars")
	}
}
