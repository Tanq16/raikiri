package utils

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

var (
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(4)).Bold(true)  // blue
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(2)).Bold(true)  // green
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(1)).Bold(true)  // red
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(3)).Bold(true)  // yellow
	genericStyle = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(5)).Bold(true)  // magenta
)

func PrintInfo(msg string) {
	fmt.Println(infoStyle.Render("[INFO]") + " " + msg)
}

func PrintSuccess(msg string) {
	fmt.Println(successStyle.Render("[OK]") + " " + msg)
}

func PrintError(msg string) {
	fmt.Fprintln(os.Stderr, errorStyle.Render("[ERROR]") + " " + msg)
}

func PrintFatal(msg string) {
	fmt.Fprintln(os.Stderr, errorStyle.Render("[FATAL]") + " " + msg)
	os.Exit(1)
}

func PrintWarn(msg string) {
	fmt.Println(warnStyle.Render("[WARN]") + " " + msg)
}

func PrintGeneric(label, msg string) {
	fmt.Println(genericStyle.Render("["+label+"]") + " " + msg)
}
