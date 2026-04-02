package tui

import (
	"context"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"neabrain/internal/app"
)

// Run starts the Bubble Tea TUI.
func Run(ctx context.Context, appInstance *app.App, out io.Writer) int {
	model := newModel(ctx, appInstance)
	p := tea.NewProgram(model, tea.WithContext(ctx))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(out, "tui error:", err)
		return 1
	}
	return 0
}
