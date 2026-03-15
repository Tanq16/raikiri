package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

// AppVersion is set at build time via ldflags.
var AppVersion = "dev-build"

var rootCmd = &cobra.Command{
	Use:               "raikiri",
	Short:             "A fast, simple, self-hosted media server",
	Version:           AppVersion,
	CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
}

func init() {
	log.SetFlags(log.Ldate | log.Ltime)
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
