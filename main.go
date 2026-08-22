// gh-followers is a gh CLI extension that lists your GitHub followers,
// sortable by their top repository's stars or by their own follower count.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"

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

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	footerStyle = lipgloss.NewStyle().Faint(true).Padding(0, 1)
	errStyle    = lipgloss.NewStyle().Bold(true).Padding(1, 1)
)

type model struct {
	followers []follower
	table     table.Model
	mode      sortMode
	loading   bool
	err       error
}

func initialModel() model {
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
	return model{table: t, loading: true}
}

func (m model) Init() tea.Cmd { return fetch }

func (m *model) refreshRows() {
	sortBy(m.followers, m.mode)
	rows := make([]table.Row, len(m.followers))
	for i, f := range m.followers {
		mark := "·"
		if f.Following {
			mark = "✓"
		}
		rows[i] = table.Row{
			mark, f.Login, f.Name,
			fmt.Sprint(f.Stars), f.TopRepo, fmt.Sprint(f.Followers),
		}
	}
	m.table.SetRows(rows)
	m.table.SetCursor(0)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.loading = false
		m.followers = msg.followers
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
		// ponytail: `s` cycles instead of one key per mode, because the table's
		// default keymap already owns f/b/d/u/g/G for scrolling.
		case "s":
			if m.loading || m.err != nil {
				return m, nil
			}
			if m.mode == byStars {
				m.mode = byFollowers
			} else {
				m.mode = byStars
			}
			m.refreshRows()
			return m, nil
		case "o":
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

func (m model) View() string {
	if m.err != nil {
		return errStyle.Render("Error: "+m.err.Error()) + "\n" +
			footerStyle.Render("q quit") + "\n"
	}
	if m.loading {
		return "\n" + headerStyle.Render("Loading followers…") + "\n"
	}
	header := fmt.Sprintf("%d followers · sorted by %s", len(m.followers), m.mode)
	return "\n" + headerStyle.Render(header) + "\n" +
		m.table.View() + "\n" +
		footerStyle.Render("s: sort · o: open profile · q: quit") + "\n"
}

func main() {
	if _, err := tea.NewProgram(initialModel(), tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
