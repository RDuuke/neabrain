package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"neabrain/internal/domain"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err.Error()
		return m, nil

	case searchResultsMsg:
		m.loading = false
		m.results = msg.results
		m.cursor = 0
		m.scroll = 0
		m.screen = screenResults
		return m, nil

	case projectsLoadedMsg:
		m.loading = false
		m.projects = msg.projects
		m.cursor = 0
		m.scroll = 0
		m.screen = screenProjects
		return m, nil

	case tea.KeyMsg:
		// Clear error on any key.
		if m.err != "" {
			m.err = ""
			return m, nil
		}
		switch m.screen {
		case screenDashboard:
			return m.updateDashboard(msg)
		case screenSearch:
			return m.updateSearch(msg)
		case screenResults:
			return m.updateResults(msg)
		case screenDetail:
			return m.updateDetail(msg)
		case screenProjects:
			return m.updateProjects(msg)
		}
	}
	return m, nil
}

// ── Dashboard ───────────────────────────────────────────────────────────────

func (m Model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.cursor < len(dashboardMenu)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter", " ":
		return m.selectDashboardItem()
	case "s", "/":
		return m.goSearch()
	case "p":
		return m.goProjects()
	}
	return m, nil
}

func (m Model) selectDashboardItem() (tea.Model, tea.Cmd) {
	if m.cursor >= 0 && m.cursor < len(dashboardMenu) {
		switch dashboardMenu[m.cursor].action {
		case screenSearch:
			return m.goSearch()
		case screenProjects:
			return m.goProjects()
		}
	}
	return m, nil
}

func (m Model) goSearch() (tea.Model, tea.Cmd) {
	m.screen = screenSearch
	m.searchInput.SetValue("")
	m.err = ""
	return m, textinput.Blink
}

func (m Model) goProjects() (tea.Model, tea.Cmd) {
	m.loading = true
	m.err = ""
	return m, m.cmdLoadProjects()
}

// ── Search ───────────────────────────────────────────────────────────────────

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.screen = screenDashboard
		m.cursor = 0
		return m, nil
	case "enter":
		query := m.searchInput.Value()
		if query == "" {
			return m, nil
		}
		m.query = query
		m.loading = true
		m.err = ""
		return m, m.cmdSearch(query)
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

// ── Results ──────────────────────────────────────────────────────────────────

func (m Model) updateResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.screen = screenSearch
		return m, textinput.Blink
	case "/", "s":
		return m.goSearch()
	case "j", "down":
		if m.cursor < len(m.results)-1 {
			m.cursor++
			m.adjustScroll()
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.adjustScroll()
		}
	case "enter":
		if m.cursor >= 0 && m.cursor < len(m.results) {
			obs := m.results[m.cursor]
			m.detail = &obs
			m.scroll = 0
			m.screen = screenDetail
		}
	}
	return m, nil
}

// ── Detail ───────────────────────────────────────────────────────────────────

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.screen = screenResults
		m.scroll = 0
	case "j", "down":
		m.scroll++
	case "k", "up":
		if m.scroll > 0 {
			m.scroll--
		}
	}
	return m, nil
}

// ── Projects ─────────────────────────────────────────────────────────────────

func (m Model) updateProjects(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.screen = screenDashboard
		m.cursor = 0
	case "j", "down":
		if m.cursor < len(m.projects)-1 {
			m.cursor++
			m.adjustScroll()
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.adjustScroll()
		}
	case "enter", "/":
		if m.cursor >= 0 && m.cursor < len(m.projects) {
			proj := m.projects[m.cursor].Name
			m.query = proj
			m.loading = true
			m.err = ""
			return m, m.cmdSearch(proj)
		}
	}
	return m, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func (m *Model) adjustScroll() {
	visible := m.visibleItems()
	if visible <= 0 {
		return
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+visible {
		m.scroll = m.cursor - visible + 1
	}
}

func (m Model) visibleItems() int {
	// Reserve lines: header(3) + footer(2) + padding(2)
	v := m.height - 7
	if v < 1 {
		return 1
	}
	return v
}

// Observation helpers used in view.
func truncate(s string, n int) string {
	// Strip newlines for single-line preview.
	out := make([]rune, 0, n)
	for _, r := range s {
		if r == '\n' || r == '\r' {
			out = append(out, ' ')
		} else {
			out = append(out, r)
		}
		if len(out) >= n {
			break
		}
	}
	result := string(out)
	if len([]rune(s)) > n {
		result += "..."
	}
	return result
}

func obsProject(obs domain.Observation) string {
	if obs.Project == "" {
		return "—"
	}
	return obs.Project
}
