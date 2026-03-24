package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Shared scanner so sequential PromptInput calls each read the next line
// instead of draining all of stdin on the first call.
var stdinScanner *bufio.Scanner

func getStdinScanner() *bufio.Scanner {
	if stdinScanner == nil {
		stdinScanner = bufio.NewScanner(os.Stdin)
	}
	return stdinScanner
}

func ReadPipedLine() string {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice != 0 {
		return ""
	}
	scanner := getStdinScanner()
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func ReadPipedInput() string {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice != 0 {
		return ""
	}
	scanner := getStdinScanner()
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func PromptInput(prompt string, placeholder string) string {
	if GlobalForAIFlag {
		return ReadPipedLine()
	}

	if placeholder != "" {
		fmt.Printf("%s [%s]: ", prompt, placeholder)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	scanner := getStdinScanner()
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}
