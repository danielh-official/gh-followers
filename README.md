# gh-followers

A [`gh`](https://cli.github.com) extension that lists your GitHub followers in a
terminal table, sorted by their top repository's stars or by their own follower
count. Results are cached locally, so it starts instantly without hitting the
API.

```
42 followers · sorted by top repo stars · All · cached 3h ago

    User              Name              Stars    Top Repo                    Followers
  ✓ octocat           The Octocat       2841     octocat/Hello-World         14203
  · someone           Some One          97       someone/dotfiles            311

  r: refresh · F: filter · s: sort · o: open profile · q: quit
```

## Install

```sh
gh extension install danielh-official/gh-followers
```

## Usage

```sh
gh followers
```

| Key | Action |
| --- | --- |
| `r` | Refresh from the API and update the cache |
| `F` | Cycle filter: All → Followed → Not Followed |
| `s` | Toggle sort: top repo stars ⇄ follower count |
| `o` | Open the selected user's profile in a browser |
| `↑`/`↓`, `pgup`/`pgdn`, `g`/`G` | Move around the table |
| `q`, `ctrl+c` | Quit |

The `✓` in the first column means you follow them back; `·` means you don't.
"Top Repo" is the follower's highest-starred non-fork repo they own, or `-` if
they have none.

`F` is shift-f, because plain `f` is already page-down in the table.

## Caching

Followers are cached at `followers.json` under your OS cache directory
(`~/Library/Caches/gh-followers/` on macOS, `~/.cache/gh-followers/` on Linux).

Startup reads that file and makes **no API call at all** — the list appears
instantly. It only hits the API on a cold start, when the cache is missing or
unreadable, or when you press `r`. The header shows how old the data is
(`cached 3h ago`), so stale results are never silent.

To start fresh, delete the file. A corrupt or truncated cache is treated as no
cache, so there is nothing to repair.

## How it works

One `gh api graphql --paginate` call fetches every follower, so there's no auth
code and no token handling — `gh` is already installed and signed in when it
runs an extension.

`o` shells out to `open`, so opening profiles works on macOS only.

## Development

```sh
go test ./...
go build && ./gh-followers
```
