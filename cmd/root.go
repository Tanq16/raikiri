package cmd

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/tanq16/raikiri/internal/utils"
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
	cobra.OnInitialize(setupLogs)
	rootCmd.PersistentFlags().BoolVar(&utils.GlobalDebugFlag, "debug", false, "Enable debug logging")
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
}

func setupLogs() {
	level := zerolog.InfoLevel
	if utils.GlobalDebugFlag {
		level = zerolog.DebugLevel
	}
	log.Logger = zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime},
	).With().Timestamp().Logger().Level(level)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
