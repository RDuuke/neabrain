package tui

import (
	"fmt"
	"strings"

	"neabrain/internal/domain"
)

func (m Model) View() string {
	if m.err != "" {
		return styleApp.Render(styleError.Render("Error: "+m.err) + "\n\n" + styleHelp.Render("Press any key to continue"))
	}
	if m.loading {
		return styleApp.Render(styleSubtle.Render("Loading..."))
	}

	switch m.screen {
	case screenDashboard:
		return m.viewDashboard()
	case screenSearch:
		return m.viewSearch()
	case screenResults:
		return m.viewResults()
	case screenDetail:
		return m.viewDetail()
	case screenProjects:
		return m.viewProjects()
	}
	return ""
}

// ── Dashboard ────────────────────────────────────────────────────────────────

func (m Model) viewDashboard() string {
	var b strings.Builder

	b.WriteString(styleLogo.Render("NeaBrain") + "  " + styleSubtle.Render("memory system") + "\n\n")

	for i, item := range dashboardMenu {
		cursor := "  "
		style := styleTitle
		if i == m.cursor {
			cursor = styleCursor.Render("▸ ")
			style = styleSelected
		}
		b.WriteString(cursor + style.Render(item.label) + "\n")
	}

	b.WriteString("\n" + styleHelp.Render("j/k navigate • enter select • s search • p projects • q quit"))
	return styleApp.Render(b.String())
}

// ── Search ───────────────────────────────────────────────────────────────────

func (m Model) viewSearch() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("Search") + "\n\n")
	b.WriteString(m.searchInput.View() + "\n\n")
	b.WriteString(styleHelp.Render("enter to search • esc back"))
	return styleApp.Render(b.String())
}

// ── Results ──────────────────────────────────────────────────────────────────

func (m Model) viewResults() string {
	var b strings.Builder

	header := fmt.Sprintf("Results for %q — %d found", m.query, len(m.results))
	b.WriteString(styleTitle.Render(header) + "\n\n")

	if len(m.results) == 0 {
		b.WriteString(styleSubtle.Render("No results.") + "\n")
	} else {
		visible := m.visibleItems()
		end := m.scroll + visible
		if end > len(m.results) {
			end = len(m.results)
		}

		for i := m.scroll; i < end; i++ {
			b.WriteString(m.renderObsItem(i, m.results[i]))
		}

		if len(m.results) > visible {
			b.WriteString("\n" + styleSubtle.Render(fmt.Sprintf("showing %d-%d of %d", m.scroll+1, end, len(m.results))))
		}
	}

	b.WriteString("\n" + styleHelp.Render("j/k navigate • enter detail • / new search • esc back"))
	return styleApp.Render(b.String())
}

func (m Model) renderObsItem(idx int, obs domain.Observation) string {
	cursor := "  "
	titleStyle := styleTitle
	if idx == m.cursor {
		cursor = styleCursor.Render("▸ ")
		titleStyle = styleSelected
	}

	idStr := styleID.Render(obs.ID[:8])
	proj := styleProject.Render(obsProject(obs))
	title := titleStyle.Render(truncate(obs.Content, 60))
	preview := styleSubtle.Render(truncate(obs.Content, 80))

	return cursor + idStr + "  " + proj + "\n" +
		"    " + title + "\n" +
		"    " + preview + "\n"
}

// ── Detail ───────────────────────────────────────────────────────────────────

func (m Model) viewDetail() string {
	if m.detail == nil {
		return ""
	}
	obs := m.detail

	var b strings.Builder
	b.WriteString(styleTitle.Render("Observation") + "  " + styleID.Render(obs.ID) + "\n\n")

	// Metadata row
	b.WriteString(styleSubtle.Render("project: ") + styleProject.Render(obsProject(*obs)))
	if obs.TopicKey != "" {
		b.WriteString("  " + styleSubtle.Render("topic: ") + styleTag.Render(obs.TopicKey))
	}
	b.WriteString("\n")
	if len(obs.Tags) > 0 {
		for _, t := range obs.Tags {
			b.WriteString(styleTag.Render("#"+t) + " ")
		}
		b.WriteString("\n")
	}
	b.WriteString(styleSubtle.Render(obs.CreatedAt.Local().Format("2006-01-02 15:04")) + "\n\n")

	// Content with scroll
	lines := strings.Split(obs.Content, "\n")
	maxWidth := m.width - 4
	if maxWidth < 20 {
		maxWidth = 80
	}
	visible := m.height - 10
	if visible < 5 {
		visible = 5
	}

	start := m.scroll
	if start > len(lines) {
		start = len(lines)
	}
	end := start + visible
	if end > len(lines) {
		end = len(lines)
	}

	for _, line := range lines[start:end] {
		if len(line) > maxWidth {
			line = line[:maxWidth] + "…"
		}
		b.WriteString(line + "\n")
	}

	if len(lines) > visible {
		b.WriteString("\n" + styleSubtle.Render(fmt.Sprintf("line %d/%d", start+1, len(lines))))
	}

	b.WriteString("\n" + styleHelp.Render("j/k scroll • esc back"))
	return styleApp.Render(b.String())
}

// ── Projects ──────────────────────────────────────────────────────────────────

func (m Model) viewProjects() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("Projects") + "\n\n")

	if len(m.projects) == 0 {
		b.WriteString(styleSubtle.Render("No projects yet.") + "\n")
	} else {
		visible := m.visibleItems()
		end := m.scroll + visible
		if end > len(m.projects) {
			end = len(m.projects)
		}

		for i := m.scroll; i < end; i++ {
			p := m.projects[i]
			cursor := "  "
			nameStyle := styleTitle
			if i == m.cursor {
				cursor = styleCursor.Render("▸ ")
				nameStyle = styleSelected
			}
			count := styleCount.Render(fmt.Sprintf("(%d)", p.Count))
			b.WriteString(cursor + nameStyle.Render(p.Name) + "  " + count + "\n")
		}

		if len(m.projects) > visible {
			b.WriteString("\n" + styleSubtle.Render(fmt.Sprintf("showing %d-%d of %d", m.scroll+1, end, len(m.projects))))
		}
	}

	b.WriteString("\n" + styleHelp.Render("j/k navigate • enter search project • esc back"))
	return styleApp.Render(b.String())
}
