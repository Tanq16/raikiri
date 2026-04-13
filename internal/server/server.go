package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed static
var staticFiles embed.FS

type Config struct {
	Port      int
	MediaPath string
	MusicPath string
	CachePath string
}

type Server struct {
	config       Config
	mux          *http.ServeMux
	activeStreams map[string]*exec.Cmd
	streamMutex  sync.Mutex
}

func New(cfg Config) *Server {
	return &Server{
		config:       cfg,
		mux:          http.NewServeMux(),
		activeStreams: make(map[string]*exec.Cmd),
	}
}

func (s *Server) Setup() error {
	s.mux.HandleFunc("/api/list", s.HandleList)
	s.mux.HandleFunc("/api/stream", s.HandleStreamStart)
	s.mux.HandleFunc("/api/stop-stream", s.HandleStreamStop)
	s.mux.HandleFunc("/api/queue.m3u8", s.HandleQueueManifest)
	s.mux.HandleFunc("/api/queue-meta", s.HandleQueueMeta)
	s.mux.HandleFunc("/api/upload", s.HandleUpload)
	s.mux.HandleFunc("/content/", s.HandleContent)

	hlsHandler := s.makeHLSHandler()
	s.mux.Handle("/hls/", http.StripPrefix("/hls/", hlsHandler))
	s.mux.Handle("/api/hls/", http.StripPrefix("/api/hls/", hlsHandler))

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("failed to prepare static assets: %w", err)
	}
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	s.mux.HandleFunc("/", s.handleIndex)

	return nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (s *Server) Run(ctx context.Context) error {
	if err := os.MkdirAll(s.config.CachePath, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	go s.cleanupOldCacheSessions(ctx)

	addr := fmt.Sprintf(":%d", s.config.Port)
	srv := &http.Server{Addr: addr, Handler: s.mux}

	go func() {
		<-ctx.Done()
		log.Printf("INFO [server] shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("INFO [server] raikiri running media=%s music=%s cache=%s port=%d", s.config.MediaPath, s.config.MusicPath, s.config.CachePath, s.config.Port)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) getRoot(mode string) string {
	if mode == "music" {
		return s.config.MusicPath
	}
	return s.config.MediaPath
}

func (s *Server) cleanupOldCacheSessions(ctx context.Context) {
	for {
		now := time.Now()
		next3AM := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
		if now.After(next3AM) {
			next3AM = next3AM.Add(24 * time.Hour)
		}

		duration := next3AM.Sub(now)
		log.Printf("INFO [server] cache cleanup scheduled at=%s in=%s", next3AM.Format("2006-01-02 15:04:05"), duration.Round(time.Second).String())

		select {
		case <-ctx.Done():
			return
		case <-time.After(duration):
		}

		log.Printf("INFO [server] starting cache cleanup")
		cutoffTime := time.Now().Add(-3 * 24 * time.Hour)

		entries, err := os.ReadDir(s.config.CachePath)
		if err != nil {
			log.Printf("ERROR [server] error reading cache directory: %v", err)
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
				log.Printf("INFO [server] removing old cache directory dir=%s created=%s", entry.Name(), dirTime.Format("2006-01-02 15:04:05"))
				if err := os.RemoveAll(dirPath); err != nil {
					log.Printf("ERROR [server] error removing directory dir=%s: %v", entry.Name(), err)
				} else {
					removedCount++
				}
			}
		}
		log.Printf("INFO [server] cache cleanup complete removed=%d", removedCount)
	}
}
