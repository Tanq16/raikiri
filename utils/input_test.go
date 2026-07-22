package utils

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestParseSelectIndex(t *testing.T) {
	tests := []struct {
		name string
		line string
		n    int
		want int
	}{
		{"empty", "", 3, -1},
		{"whitespace only", "   ", 3, -1},
		{"first", "1", 3, 0},
		{"last", "3", 3, 2},
		{"below range", "0", 3, -1},
		{"above range", "4", 3, -1},
		{"negative", "-1", 3, -1},
		{"non-numeric", "abc", 3, -1},
		{"float", "2.5", 3, -1},
		{"trailing space trimmed", " 2 ", 3, 1},
		{"no options", "1", 0, -1},
		{"single option", "1", 1, 0},
		{"single option out of range", "2", 1, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSelectIndex(tt.line, tt.n); got != tt.want {
				t.Fatalf("parseSelectIndex(%q, %d) = %d, want %d", tt.line, tt.n, got, tt.want)
			}
		})
	}
}

func press(m selectModel, key string) selectModel {
	var msg tea.KeyPressMsg
	switch key {
	case "up":
		msg = tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		msg = tea.KeyPressMsg{Code: tea.KeyDown}
	case "enter":
		msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		msg = tea.KeyPressMsg{Code: tea.KeyEsc}
	default:
		msg = tea.KeyPressMsg{Code: rune(key[0]), Text: key}
	}
	updated, _ := m.Update(msg)
	return updated.(selectModel)
}

func TestSelectModelClampsAtTop(t *testing.T) {
	m := selectModel{options: []string{"a", "b", "c"}, choice: -1}
	m = press(m, "up")
	if m.cursor != 0 {
		t.Fatalf("cursor = %d after up at top, want 0", m.cursor)
	}
}

func TestSelectModelClampsAtBottom(t *testing.T) {
	m := selectModel{options: []string{"a", "b", "c"}, choice: -1}
	for range 5 {
		m = press(m, "down")
	}
	if m.cursor != 2 {
		t.Fatalf("cursor = %d after 5 downs, want 2 (len-1)", m.cursor)
	}
}

func TestSelectModelVimKeys(t *testing.T) {
	m := selectModel{options: []string{"a", "b", "c"}, choice: -1}
	m = press(m, "j")
	m = press(m, "j")
	if m.cursor != 2 {
		t.Fatalf("cursor = %d after two j, want 2", m.cursor)
	}
	m = press(m, "k")
	if m.cursor != 1 {
		t.Fatalf("cursor = %d after k, want 1", m.cursor)
	}
}

func TestSelectModelEnterSelects(t *testing.T) {
	m := selectModel{options: []string{"a", "b", "c"}, choice: -1}
	m = press(m, "down")
	m = press(m, "enter")
	if !m.done || m.choice != 1 {
		t.Fatalf("after enter on index 1: done=%v choice=%d, want done=true choice=1", m.done, m.choice)
	}
}

func TestSelectModelCancelKeys(t *testing.T) {
	for _, key := range []string{"esc", "q"} {
		t.Run(key, func(t *testing.T) {
			m := selectModel{options: []string{"a", "b", "c"}, choice: -1}
			m = press(m, "down")
			m = press(m, key)
			if !m.done || m.choice != -1 {
				t.Fatalf("after %q: done=%v choice=%d, want done=true choice=-1", key, m.done, m.choice)
			}
		})
	}
}
