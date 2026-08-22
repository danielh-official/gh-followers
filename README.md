# gh-followers

A [`gh`](https://cli.github.com) extension that lists your GitHub followers in a
terminal table, sorted by their top repository's stars or by their own follower
count.

```
42 followers · sorted by top repo stars

    User              Name              Stars    Top Repo                    Followers
  ✓ octocat           The Octocat       2841     octocat/Hello-World         14203
  · someone           Some One          97       someone/dotfiles            311

  s sort · enter open profile · q quit
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
| `s` | Toggle sort: top repo stars ⇄ follower count |
| `enter` | Open the selected user's profile in a browser |
| `↑`/`↓`, `pgup`/`pgdn`, `g`/`G` | Move around the table |
| `q`, `ctrl+c` | Quit |

The `✓` in the first column means you follow them back; `·` means you don't.
"Top Repo" is the follower's highest-starred non-fork repo they own, or `-` if
they have none.

## How it works

One `gh api graphql --paginate` call fetches every follower, so there's no auth
code and no token handling — `gh` is already installed and signed in when it
runs an extension.

`enter` shells out to `open`, so opening profiles works on macOS only.

## Development

```sh
go test ./...
go build && ./gh-followers
```
