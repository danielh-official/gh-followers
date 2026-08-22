# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A `gh` CLI extension (`gh followers`): a Bubble Tea TUI listing the authenticated
user's GitHub followers, sortable by their top repo's stars or their own follower
count. Single `main` package, two source files.

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

Split by concern, not by grammar: `github.go` knows GitHub exists, `main.go` knows
the terminal exists. The boundary is `loadedMsg` — put new code on the side that
matches what it talks to, and resist reintroducing a `types.go`/`helpers.go` junk
drawer that scatters one concept across files.

Data flows one way: `fetch` → `loadedMsg` → `flatten` → `sortBy` → `table.Rows`.

- **`github.go`** — the wire. `fetch` shells out to
  `gh api graphql --paginate --slurp` rather than importing `go-gh`, because a
  `gh` extension is guaranteed to run under an installed, authenticated `gh`. That
  means **no auth code exists in this repo**, and it should stay that way unless
  the tool has to run outside `gh`. `followersQuery`'s `$endCursor` variable name
  and `pageInfo { hasNextPage endCursor }` block are a contract with `--paginate`;
  renaming either silently breaks pagination to one page. `page` mirrors one
  element of the JSON array `--slurp` emits, so the response decodes as `[]page`,
  not a single object; that nested shape never escapes `flatten`, which returns
  the flat `follower`.
- **`main.go`** — the TUI, plus `sortMode` and its constants, `String()`, and
  `sortBy` all in one place. `Update` has a **value** receiver while `refreshRows`
  has a **pointer** receiver; this works only because `Update` returns `m` by
  value. Any new state mutation must follow the same pattern or the change is
  discarded.
- Unhandled keys fall through to `m.table.Update(msg)`, which is why the table's
  own keymap (`f`/`b`/`d`/`u`/`g`/`G`) is off limits — sorting cycles on a single
  `s` instead of one key per mode.

## Testing

Tests drive the model directly (`initialModel().Update(...)`) with a JSON fixture
of the real `--slurp` shape, so there is no pty and no network. `gh` is never
invoked from tests. The fixture deliberately includes a follower with empty
`repositories.nodes` (a user with no owned non-fork repos) — the case that panics
if `flatten` indexes `Nodes[0]` unguarded.

## Conventions

- `// ponytail:` comments mark deliberate simplifications and name the upgrade
  path. Keep them when touching that code; add one when taking a shortcut with a
  known ceiling (e.g. `enter` uses `open`, so it is macOS-only).
- No new dependencies beyond the three charmbracelet ones without a reason the
  stdlib or `gh` itself cannot cover.
