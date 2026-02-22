package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func plog() *zerolog.Logger {
	l := log.With().Str("package", "server").Logger()
	return &l
}

//go:embed static
var staticFiles embed.FS

// Config holds the server configuration values.
type Config struct {
	Port      int
	MediaPath string
	MusicPath string
	CachePath string
}

// Server is the Raikiri HTTP server.
type Server struct {
	config        Config
	mux           *http.ServeMux
	activeStreams  map[string]*exec.Cmd
	streamMutex   sync.Mutex
}

// New creates a new Server with the given configuration.
func New(cfg Config) *Server {
	return &Server{
		config:       cfg,
		mux:          http.NewServeMux(),
		activeStreams: make(map[string]*exec.Cmd),
	}
}

// Setup registers all routes and returns the server for chaining.
func (s *Server) Setup() *Server {
	s.mux.HandleFunc("/api/list", s.HandleList)
	s.mux.HandleFunc("/api/stream", s.HandleStreamStart)
	s.mux.HandleFunc("/api/stop-stream", s.HandleStreamStop)
	s.mux.HandleFunc("/api/upload", s.HandleUpload)
	s.mux.HandleFunc("/content/", s.HandleContent)

	hlsHandler := s.makeHLSHandler()
	s.mux.Handle("/hls/", http.StripPrefix("/hls/", hlsHandler))
	s.mux.Handle("/api/hls/", http.StripPrefix("/api/hls/", hlsHandler))

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	s.mux.Handle("/", http.FileServer(http.FS(sub)))

	return s
}

// Run starts the HTTP server and the cache cleanup goroutine.
// It blocks until ctx is cancelled, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	if err := os.MkdirAll(s.config.CachePath, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	go s.cleanupOldCacheSessions(ctx)

	addr := fmt.Sprintf(":%d", s.config.Port)
	srv := &http.Server{Addr: addr, Handler: s.mux}

	go func() {
		<-ctx.Done()
		plog().Info().Msg("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	plog().Info().
		Str("media", s.config.MediaPath).
		Str("music", s.config.MusicPath).
		Str("cache", s.config.CachePath).
		Int("port", s.config.Port).
		Msg("raikiri running")

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// getRoot returns the filesystem root for the given mode.
func (s *Server) getRoot(mode string) string {
	if mode == "music" {
		return s.config.MusicPath
	}
	return s.config.MediaPath
}

// cleanupOldCacheSessions removes cache session directories older than 3 days.
func (s *Server) cleanupOldCacheSessions(ctx context.Context) {
	for {
		now := time.Now()
		next3AM := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
		if now.After(next3AM) {
			next3AM = next3AM.Add(24 * time.Hour)
		}

		duration := next3AM.Sub(now)
		plog().Info().
			Str("scheduled", next3AM.Format("2006-01-02 15:04:05")).
			Str("in", duration.Round(time.Second).String()).
			Msg("cache cleanup scheduled")

		select {
		case <-ctx.Done():
			return
		case <-time.After(duration):
		}

		plog().Info().Msg("starting cache cleanup")
		cutoffTime := time.Now().Add(-3 * 24 * time.Hour)

		entries, err := os.ReadDir(s.config.CachePath)
		if err != nil {
			plog().Error().Err(err).Msg("error reading cache directory")
			continue
		}

		removedCount := 0
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dirPath := filepath.Join(s.config.CachePath, entry.Name())
			var dirTime time.Time
			if after, ok := strings.CutPrefix(entry.Name(), "s_"); ok {
				if unixNano, err := strconv.ParseInt(after, 10, 64); err == nil {
					dirTime = time.Unix(0, unixNano)
				}
			}

			if dirTime.Before(cutoffTime) {
				plog().Info().
					Str("dir", entry.Name()).
					Str("created", dirTime.Format("2006-01-02 15:04:05")).
					Msg("removing old cache directory")
				if err := os.RemoveAll(dirPath); err != nil {
					plog().Error().Err(err).Str("dir", entry.Name()).Msg("error removing directory")
				} else {
					removedCount++
				}
			}
		}
		plog().Info().Int("removed", removedCount).Msg("cache cleanup complete")
	}
}
