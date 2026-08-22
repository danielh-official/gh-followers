# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A `gh` CLI extension (`gh followers`): a Bubble Tea TUI listing the authenticated
user's GitHub followers, sortable by their top repo's stars or their own follower
count, with a local cache so startup makes no API call. Single `main`
package, three source files.

## Commands

```sh
go test ./...                                   # all tests
go test -run TestSortKeyRebuildsRows ./...      # one test
go vet ./...
go build && ./gh-followers                      # run locally (needs gh installed + authed)
gh extension install .                          # symlink this dir as the local `gh followers`
```

The built `gh-followers` binary is gitignored — `gh extension install .` symlinks
to it, and releases build it in CI.

## Architecture

Split by concern, not by grammar: `github.go` knows GitHub exists, `cache.go`
knows the disk exists, `main.go` knows the terminal exists. The boundary is
`loadedMsg` — put new code on the side that matches what it talks to, and resist
reintroducing a `types.go`/`helpers.go` junk drawer that scatters one concept
across files.

Two entry paths into the same state:

- **Warm start** — `main()` reads the cache, `initialModel` renders it, `Init()`
  returns `nil`. **No API call.** That is the feature; `TestWarmStartSkipsFetch`
  guards it.
- **Cold start or `r`** — `fetch` → `loadedMsg` → `flatten` → `sortBy` →
  `table.Rows`, and `fetch` writes the cache on its way through.

By file:

- **`github.go`** — the wire. `fetch` shells out to
  `gh api graphql --paginate --slurp` rather than importing `go-gh`, because a
  `gh` extension is guaranteed to run under an installed, authenticated `gh`. That
  means **no auth code exists in this repo**, and it should stay that way unless
  the tool has to run outside `gh`. `followersQuery`'s `$endCursor` variable name
  and `pageInfo { hasNextPage endCursor }` block are a contract with `--paginate`;
  renaming either silently breaks pagination to one page. `page` mirrors one
  element of the JSON array `--slurp` emits, so the response decodes as `[]page`,
  not a single object; that nested shape never escapes `flatten`, which returns
  the flat `follower`. `fetch` also writes the cache after a successful decode —
  it is already off the render loop, so this needs no extra plumbing.
- **`cache.go`** — `~/Library/Caches/gh-followers/followers.json` via
  `os.UserCacheDir()`. `readCache` returns `ok == false` for *every* failure
  (missing, corrupt, truncated, empty): a bad cache is a miss, never a user-facing
  error. Writes are not atomic, and that is deliberate — a torn file reads back as
  a miss and the next fetch overwrites it. `readCache`/`writeCache` take explicit
  paths so tests use `t.TempDir()`; `cachePath()` is only called by `main` and
  `fetch`.
- **`main.go`** — the TUI, plus `sortMode` and `filterMode` with their
  constants, `String()`, `matches`, and `sortBy` — each mode lives in one place. `Update` has a **value** receiver while `refreshRows`
  has a **pointer** receiver; this works only because `Update` returns `m` by
  value. Any new state mutation must follow the same pattern or the change is
  discarded. `initialModel` takes a `cacheFile` rather than reading one, so no I/O
  happens in the constructor and tests never touch the real cache directory.
- `enter` reads the login from `m.table.SelectedRow()[1]` — the row's own text,
  not an index into `m.followers`. That is what makes filtering safe; do not
  "optimize" it into an index lookup.
- Sort and filter are gated on `len(m.followers) == 0`, not on `m.err`, so both
  still work over stale cached rows after a failed refresh.
- Unhandled keys fall through to `m.table.Update(msg)`, which is why the table's
  own keymap (`f`/`b`/`d`/`u`/`g`/`G`) is off limits — sorting cycles on a single
  `s` instead of one key per mode, and the filter is `F` (shift-f) because plain
  `f` is the table's PageDown.

## Testing

Tests drive the model directly (`initialModel(cacheFile{}).Update(...)`, via the
`modelFromFixture` helper) with a JSON fixture
of the real `--slurp` shape, so there is no pty and no network. `gh` is never
invoked from tests. The fixture deliberately includes a follower with empty
`repositories.nodes` (a user with no owned non-fork repos) — the case that panics
if `flatten` indexes `Nodes[0]` unguarded. Cache tests write into `t.TempDir()`,
so they never read or clobber the developer's own cached followers.

## Conventions

- `// ponytail:` comments mark deliberate simplifications and name the upgrade
  path. Keep them when touching that code; add one when taking a shortcut with a
  known ceiling (e.g. `enter` uses `open`, so it is macOS-only).
- No new dependencies beyond the three charmbracelet ones without a reason the
  stdlib or `gh` itself cannot cover.
