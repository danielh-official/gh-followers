// gh-followers is a gh CLI extension that lists your GitHub followers,
// sortable by their top repository's stars or by their own follower count.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type sortMode int

const (
	byStars sortMode = iota
	byFollowers
)

func (s sortMode) String() string {
	if s == byStars {
		return "top repo stars"
	}
	return "follower count"
}

func sortBy(fs []follower, mode sortMode) {
	sort.SliceStable(fs, func(i, j int) bool {
		if mode == byStars {
			return fs[i].Stars > fs[j].Stars
		}
		return fs[i].Followers > fs[j].Followers
	})
}

type filterMode int

const (
	filterAll filterMode = iota
	filterFollowed
	filterNotFollowed
	numFilters
)

func (f filterMode) String() string {
	switch f {
	case filterFollowed:
		return "Followed"
	case filterNotFollowed:
		return "Not Followed"
	}
	return "All"
}

func (f filterMode) matches(x follower) bool {
	switch f {
	case filterFollowed:
		return x.Following
	case filterNotFollowed:
		return !x.Following
	}
	return true
}

// ago renders cache age; time.Since's own String() is unreadable ("3h14m22.6s").
func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours())/24)
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	footerStyle = lipgloss.NewStyle().Faint(true).Padding(0, 1)
	errStyle    = lipgloss.NewStyle().Bold(true).Padding(1, 1)
)

type model struct {
	followers []follower
	table     table.Model
	mode      sortMode
	filter    filterMode
	fetchedAt time.Time
	loading   bool
	err       error
}

// initialModel takes the cache rather than reading it, so no I/O happens here
// and tests never touch the real cache directory.
func initialModel(c cacheFile) model {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "", Width: 1},
			{Title: "User", Width: 24},
			{Title: "Name", Width: 24},
			{Title: "Stars", Width: 7},
			{Title: "Top Repo", Width: 34},
			{Title: "Followers", Width: 9},
		}),
		table.WithFocused(true),
		table.WithHeight(20),
	)
	s := table.DefaultStyles()
	s.Selected = s.Selected.Bold(true)
	t.SetStyles(s)

	m := model{table: t}
	if len(c.Followers) > 0 {
		m.followers = c.Followers
		m.fetchedAt = c.FetchedAt
		m.refreshRows()
	} else {
		m.loading = true
	}
	return m
}

// Init issues no command on a cache warm start — skipping that API call is the
// whole point of the cache.
func (m model) Init() tea.Cmd {
	if m.loading {
		return fetch
	}
	return nil
}

func (m *model) refreshRows() {
	sortBy(m.followers, m.mode)
	rows := make([]table.Row, 0, len(m.followers))
	for _, f := range m.followers {
		if !m.filter.matches(f) {
			continue
		}
		mark := "·"
		if f.Following {
			mark = "✓"
		}
		rows = append(rows, table.Row{
			mark, f.Login, f.Name,
			fmt.Sprint(f.Stars), f.TopRepo, fmt.Sprint(f.Followers),
		})
	}
	m.table.SetRows(rows)
	m.table.SetCursor(0)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.loading = false
		// Clearing err matters once r exists: a failed fetch followed by a good
		// one would otherwise leave the error screen up over valid data.
		m.err = nil
		m.followers = msg.followers
		m.fetchedAt = time.Now()
		m.refreshRows()
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			if m.loading {
				return m, nil
			}
			m.loading = true
			m.err = nil
			return m, fetch
		// ponytail: `s` and `F` cycle instead of one key per mode, because the
		// table's default keymap already owns f/b/d/u/g/G for scrolling.
		case "s":
			if len(m.followers) == 0 {
				return m, nil
			}
			if m.mode == byStars {
				m.mode = byFollowers
			} else {
				m.mode = byStars
			}
			m.refreshRows()
			return m, nil
		case "F":
			if len(m.followers) == 0 {
				return m, nil
			}
			m.filter = (m.filter + 1) % numFilters
			m.refreshRows()
			return m, nil
		case "enter":
			// The login comes from the row itself, not an index into
			// m.followers, so filtering cannot desync it. Keep it that way.
			if row := m.table.SelectedRow(); len(row) > 1 {
				// ponytail: `open` is macOS-only.
				_ = exec.Command("open", "https://github.com/"+row[1]).Start()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) header() string {
	s := fmt.Sprintf("%d followers", len(m.followers))
	if shown := len(m.table.Rows()); m.filter != filterAll {
		s += fmt.Sprintf(" · %d shown", shown)
	}
	s += fmt.Sprintf(" · sorted by %s · %s", m.mode, m.filter)
	switch {
	case m.loading:
		s += " · refreshing…"
	case !m.fetchedAt.IsZero():
		s += " · cached " + ago(m.fetchedAt)
	}
	return s
}

func (m model) View() string {
	// The full-screen loading and error states apply only when there is nothing
	// else to show; otherwise a failed refresh would throw away cached rows.
	if m.err != nil && len(m.followers) == 0 {
		return errStyle.Render("Error: "+m.err.Error()) + "\n" +
			footerStyle.Render("r retry · q quit") + "\n"
	}
	if m.loading && len(m.followers) == 0 {
		return "\n" + headerStyle.Render("Loading followers…") + "\n"
	}
	out := "\n" + headerStyle.Render(m.header()) + "\n" + m.table.View() + "\n"
	if m.err != nil {
		out += errStyle.Render("Refresh failed: "+m.err.Error()) + "\n"
	}
	return out + footerStyle.Render(
		"r refresh · F filter · s sort · enter open profile · q quit") + "\n"
}

func main() {
	var c cacheFile
	if p, err := cachePath(); err == nil {
		c, _ = readCache(p)
	}
	if _, err := tea.NewProgram(initialModel(c), tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
