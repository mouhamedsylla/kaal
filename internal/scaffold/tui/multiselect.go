package tui

import (
	"fmt"
	"strings"
)

// MultiSelectItem is one entry in a multi-select list.
type MultiSelectItem struct {
	Key         string
	Label       string
	Description string
	Selected    bool
	Disabled    bool // always selected, cannot be toggled
}

// MultiSelect holds the state of a multi-select list.
type MultiSelect struct {
	Items   []MultiSelectItem
	Cursor  int
}

func NewMultiSelect(items []MultiSelectItem) MultiSelect {
	return MultiSelect{Items: items}
}

// Toggle flips the selection of the item under the cursor.
func (m *MultiSelect) Toggle() {
	if m.Cursor < len(m.Items) && !m.Items[m.Cursor].Disabled {
		m.Items[m.Cursor].Selected = !m.Items[m.Cursor].Selected
	}
}

// Up moves the cursor up.
func (m *MultiSelect) Up() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

// Down moves the cursor down.
func (m *MultiSelect) Down() {
	if m.Cursor < len(m.Items)-1 {
		m.Cursor++
	}
}

// Selected returns the keys of all selected items.
func (m *MultiSelect) SelectedKeys() []string {
	var keys []string
	for _, item := range m.Items {
		if item.Selected || item.Disabled {
			keys = append(keys, item.Key)
		}
	}
	return keys
}

// View renders the multi-select list.
func (m *MultiSelect) View() string {
	var b strings.Builder
	for i, item := range m.Items {
		cursor := "  "
		if i == m.Cursor {
			cursor = StyleCursor.Render("▶ ")
		}

		checkbox := "○"
		labelStyle := StyleDim
		if item.Disabled {
			checkbox = StyleSuccess.Render("●")
			labelStyle = StyleSelected
		} else if item.Selected {
			checkbox = StyleSelected.Render("●")
			labelStyle = StyleSelected
		}

		label := labelStyle.Render(item.Label)
		desc := ""
		if item.Description != "" {
			desc = "  " + StyleDim.Render(item.Description)
		}

		b.WriteString(fmt.Sprintf("%s%s %s%s\n", cursor, checkbox, label, desc))
	}
	return b.String()
}
