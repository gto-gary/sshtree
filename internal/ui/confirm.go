package ui

import tea "github.com/charmbracelet/bubbletea"

// confirmDoneMsg carries a yes/no modal's result back to the root model.
type confirmDoneMsg struct{ ok bool }

// confirmModel is a trivial yes/no modal, used today for delete
// confirmation.
type confirmModel struct {
	message string
}

func newConfirm(message string) *confirmModel {
	return &confirmModel{message: message}
}

func (c *confirmModel) Update(msg tea.Msg) (*confirmModel, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "y", "enter":
			return c, func() tea.Msg { return confirmDoneMsg{ok: true} }
		case "n", "esc":
			return c, func() tea.Msg { return confirmDoneMsg{ok: false} }
		}
	}
	return c, nil
}

func (c *confirmModel) View(width, height int) string {
	// "Yes" reads as the destructive choice (error-red, matching the
	// Python original's variant="error" Yes button on delete confirmation);
	// "No" reads as the safe default.
	yes := errorStyle.Bold(true).Render("[y]es")
	no := bannerStyle.Render("[n]o")
	content := c.message + "\n\n" + yes + " / " + no + "  (enter = yes, esc = no)"
	return renderDialog(width, height, content)
}
