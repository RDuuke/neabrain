package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"neabrain/internal/app"
	"neabrain/internal/domain"
)

type screen int

const (
	screenDashboard screen = iota
	screenSearch
	screenResults
	screenDetail
	screenProjects
)

// dashboardItem represents a menu entry on the dashboard.
type dashboardItem struct {
	label  string
	action screen
}

var dashboardMenu = []dashboardItem{
	{"Search memories", screenSearch},
	{"Browse projects", screenProjects},
}

// Messages used for async data loading.

type searchResultsMsg struct{ results []domain.Observation }
type projectsLoadedMsg struct{ projects []domain.ProjectSummary }
type errMsg struct{ err error }

// Model is the central Bubble Tea state for the TUI.
type Model struct {
	app    *app.App
	ctx    context.Context
	screen screen

	width, height int

	// navigation
	cursor int
	scroll int

	// search
	searchInput textinput.Model
	query       string

	// data
	results  []domain.Observation
	detail   *domain.Observation
	projects []domain.ProjectSummary

	// status
	loading bool
	err     string
}

func newModel(ctx context.Context, appInstance *app.App) Model {
	ti := textinput.New()
	ti.Placeholder = "Type query and press Enter..."
	ti.CharLimit = 256
	ti.Width = 50

	return Model{
		app:         appInstance,
		ctx:         ctx,
		screen:      screenDashboard,
		searchInput: ti,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// Commands (async data loaders)

func (m Model) cmdSearch(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := m.app.SearchService.Search(m.ctx, query, domain.SearchFilter{
			Project: m.app.Config.DefaultProject,
		})
		if err != nil {
			return errMsg{err}
		}
		return searchResultsMsg{results}
	}
}

func (m Model) cmdLoadProjects() tea.Cmd {
	return func() tea.Msg {
		projects, err := m.app.ObservationService.ListProjects(m.ctx)
		if err != nil {
			return errMsg{err}
		}
		return projectsLoadedMsg{projects}
	}
}
