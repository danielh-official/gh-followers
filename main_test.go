package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	tm, _ := initialModel(cacheFile{}).Update(loadedMsg{flatten(pages)})
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

// modelFromFixture is the shared setup: a loaded model, no cache, no network.
func modelFromFixture(t *testing.T) model {
	t.Helper()
	var pages []page
	if err := json.Unmarshal([]byte(fixture), &pages); err != nil {
		t.Fatal(err)
	}
	tm, _ := initialModel(cacheFile{}).Update(loadedMsg{flatten(pages)})
	return tm.(model)
}

func logins(m model) []string {
	var got []string
	for _, r := range m.table.Rows() {
		got = append(got, r[1])
	}
	return got
}

// F must cycle All -> Followed -> Not Followed -> All, changing only the visible
// rows. The fixture has small (not followed) plus norepos and big (followed).
func TestFilterCycle(t *testing.T) {
	m := modelFromFixture(t)
	pressF := func() {
		tm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
		m = tm.(model)
	}

	if m.filter != filterAll || len(m.table.Rows()) != 3 {
		t.Fatalf("default filter: %v with %d rows, want All with 3", m.filter, len(m.table.Rows()))
	}

	pressF()
	if m.filter != filterFollowed {
		t.Errorf("after 1 F: got %v, want Followed", m.filter)
	}
	if got := logins(m); len(got) != 2 {
		t.Errorf("Followed: got %v, want big and norepos", got)
	}
	for _, r := range m.table.Rows() {
		if r[1] == "small" {
			t.Error("Followed included small, who is not followed")
		}
	}

	pressF()
	if got := logins(m); len(got) != 1 || got[0] != "small" {
		t.Errorf("Not Followed: got %v, want [small]", got)
	}

	// Filtering must never drop anyone from the underlying slice.
	if len(m.followers) != 3 {
		t.Errorf("m.followers shrank to %d; filtering must only affect rows", len(m.followers))
	}

	pressF()
	if m.filter != filterAll || len(m.table.Rows()) != 3 {
		t.Errorf("F did not cycle back to All: %v with %d rows", m.filter, len(m.table.Rows()))
	}
}

// A cache must survive a round trip, and every miss path must report ok == false
// rather than panicking or handing back half-decoded data.
func TestCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "followers.json")
	want := cacheFile{
		FetchedAt: time.Now().Truncate(time.Second),
		Followers: []follower{{Login: "big", Stars: 5000, TopRepo: "big/z", Following: true}},
	}
	if err := writeCache(path, want); err != nil {
		t.Fatalf("writeCache into a missing dir: %v", err)
	}

	got, ok := readCache(path)
	if !ok {
		t.Fatal("readCache did not find the file just written")
	}
	if len(got.Followers) != 1 || got.Followers[0].Login != "big" || !got.Followers[0].Following {
		t.Errorf("round trip: got %+v", got.Followers)
	}
	if !got.FetchedAt.Equal(want.FetchedAt) {
		t.Errorf("FetchedAt: got %v, want %v", got.FetchedAt, want.FetchedAt)
	}

	if _, ok := readCache(filepath.Join(t.TempDir(), "nope.json")); ok {
		t.Error("missing file reported as a hit")
	}

	corrupt := filepath.Join(t.TempDir(), "corrupt.json")
	if err := os.WriteFile(corrupt, []byte(`{"followers":[{"Login":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := readCache(corrupt); ok {
		t.Error("truncated file reported as a hit")
	}

	empty := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(empty, []byte(`{"followers":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := readCache(empty); ok {
		t.Error("empty follower list reported as a hit")
	}
}

// The whole point of the cache: a warm start must issue no command, because a
// command here means an API call. A cold start must still fetch.
func TestWarmStartSkipsFetch(t *testing.T) {
	warm := cacheFile{
		FetchedAt: time.Now(),
		Followers: []follower{{Login: "big", Stars: 5000}},
	}
	m := initialModel(warm)
	if m.loading {
		t.Error("warm start left the model in a loading state")
	}
	if m.Init() != nil {
		t.Error("warm start issued a command; startup must hit no API")
	}
	if len(m.table.Rows()) != 1 {
		t.Errorf("warm start rendered %d rows, want 1 from cache", len(m.table.Rows()))
	}

	cold := initialModel(cacheFile{})
	if !cold.loading {
		t.Error("cold start is not loading")
	}
	if cold.Init() == nil {
		t.Error("cold start issued no command; it must fetch")
	}
}
