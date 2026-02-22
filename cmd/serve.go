package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/tanq16/raikiri/internal/server"
	"github.com/tanq16/raikiri/internal/utils"
)

var serveFlags struct {
	media string
	music string
	cache string
	port  int
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Raikiri media server",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		cfg := server.Config{
			Port:      serveFlags.port,
			MediaPath: serveFlags.media,
			MusicPath: serveFlags.music,
			CachePath: serveFlags.cache,
		}

		srv := server.New(cfg).Setup()
		if err := srv.Run(ctx); err != nil {
			utils.PrintFatal(err.Error())
		}
		return nil
	},
}

func init() {
	serveCmd.Flags().StringVarP(&serveFlags.media, "media", "m", ".", "Path to media directory")
	serveCmd.Flags().StringVarP(&serveFlags.music, "music", "M", "./music", "Path to music directory")
	serveCmd.Flags().StringVarP(&serveFlags.cache, "cache", "c", "/tmp", "Path to cache directory for HLS segments")
	serveCmd.Flags().IntVarP(&serveFlags.port, "port", "p", 8080, "Port to listen on")

	rootCmd.AddCommand(serveCmd)
}
