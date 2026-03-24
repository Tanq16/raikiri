package utils

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.ANSIColor(15)).
			Padding(0, 1)

	cellStyle = lipgloss.NewStyle().
			Foreground(lipgloss.ANSIColor(7)).
			Padding(0, 1)

	borderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.ANSIColor(8))
)

func PrintTable(headers []string, rows [][]string) {
	if GlobalForAIFlag {
		printMarkdownTable(headers, rows)
		return
	}

	if len(headers) == 0 {
		return
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	sepParts := make([]string, len(widths))
	for i, w := range widths {
		sepParts[i] = strings.Repeat("─", w+2)
	}
	sep := "├" + strings.Join(sepParts, "┼") + "┤"
	top := "┌" + strings.Join(sepParts, "┬") + "┐"
	bottom := "└" + strings.Join(sepParts, "┴") + "┘"

	formatRow := func(cells []string, style lipgloss.Style) string {
		parts := make([]string, len(widths))
		for i, w := range widths {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			parts[i] = style.Render(fmt.Sprintf(" %-*s ", w, cell))
		}
		return borderStyle.Render("│") + strings.Join(parts, borderStyle.Render("│")) + borderStyle.Render("│")
	}

	fmt.Println(borderStyle.Render(top))
	fmt.Println(formatRow(headers, headerStyle))
	fmt.Println(borderStyle.Render(sep))
	for _, row := range rows {
		fmt.Println(formatRow(row, cellStyle))
	}
	fmt.Println(borderStyle.Render(bottom))
}

func printMarkdownTable(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}
	fmt.Println("| " + strings.Join(escapeCells(headers), " | ") + " |")
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	fmt.Println("| " + strings.Join(seps, " | ") + " |")
	for _, row := range rows {
		fmt.Println("| " + strings.Join(escapeCells(row), " | ") + " |")
	}
}

func escapeCells(cells []string) []string {
	escaped := make([]string, len(cells))
	for i, cell := range cells {
		escaped[i] = strings.ReplaceAll(cell, "|", "\\|")
	}
	return escaped
}
