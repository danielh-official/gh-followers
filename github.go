package main

import (
	"encoding/json"
	"fmt"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

// The $endCursor variable name and the pageInfo block are the contract that
// `gh api graphql --paginate` looks for; it walks the pages itself.
const followersQuery = `
query($endCursor: String) {
  viewer {
    followers(first: 100, after: $endCursor) {
      pageInfo { hasNextPage endCursor }
      nodes {
        login
        name
        viewerIsFollowing
        followers { totalCount }
        repositories(first: 1, ownerAffiliations: OWNER, isFork: false,
                     orderBy: {field: STARGAZERS, direction: DESC}) {
          nodes { stargazerCount nameWithOwner }
        }
      }
    }
  }
}`

// page mirrors one element of the JSON array that --slurp emits.
type page struct {
	Data struct {
		Viewer struct {
			Followers struct {
				Nodes []struct {
					Login             string `json:"login"`
					Name              string `json:"name"`
					ViewerIsFollowing bool   `json:"viewerIsFollowing"`
					Followers         struct {
						TotalCount int `json:"totalCount"`
					} `json:"followers"`
					Repositories struct {
						Nodes []struct {
							StargazerCount int    `json:"stargazerCount"`
							NameWithOwner  string `json:"nameWithOwner"`
						} `json:"nodes"`
					} `json:"repositories"`
				} `json:"nodes"`
			} `json:"followers"`
		} `json:"viewer"`
	} `json:"data"`
}

// follower is the flat domain type; the nested GraphQL shape stops at flatten.
type follower struct {
	Login     string
	Name      string
	Following bool
	Stars     int
	TopRepo   string
	Followers int
}

type loadedMsg struct{ followers []follower }
type errMsg struct{ err error }

// flatten collapses the paginated response into a flat slice. A follower with
// no owned non-fork repositories has an empty Repositories.Nodes.
func flatten(pages []page) []follower {
	var fs []follower
	for _, p := range pages {
		for _, n := range p.Data.Viewer.Followers.Nodes {
			f := follower{
				Login:     n.Login,
				Name:      n.Name,
				Following: n.ViewerIsFollowing,
				TopRepo:   "-",
				Followers: n.Followers.TotalCount,
			}
			if len(n.Repositories.Nodes) > 0 {
				f.Stars = n.Repositories.Nodes[0].StargazerCount
				f.TopRepo = n.Repositories.Nodes[0].NameWithOwner
			}
			fs = append(fs, f)
		}
	}
	return fs
}

// ponytail: shells out to the gh CLI instead of pulling in go-gh. gh is by
// definition installed and authenticated when it runs an extension, so this
// needs no auth code at all. Swap in go-gh's api.DefaultGraphQLClient() only
// if this ever has to run outside a gh extension.
func fetch() tea.Msg {
	out, err := exec.Command("gh", "api", "graphql", "--paginate", "--slurp",
		"-f", "query="+followersQuery).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return errMsg{fmt.Errorf("gh api graphql: %s", ee.Stderr)}
		}
		return errMsg{fmt.Errorf("running gh: %w", err)}
	}
	var pages []page
	if err := json.Unmarshal(out, &pages); err != nil {
		return errMsg{fmt.Errorf("parsing response: %w", err)}
	}
	return loadedMsg{flatten(pages)}
}
