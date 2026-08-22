package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const historyLimit = 100

type HistoryEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Type  string `json:"type"`
	Thumb string `json:"thumb"`
}

type historyStore struct {
	Files []HistoryEntry `json:"files"`
	Music []HistoryEntry `json:"music"`
}

func (s *Server) historyPath() string {
	return filepath.Join(s.config.CachePath, "history.json")
}

func (s *Server) readHistory() historyStore {
	data, err := os.ReadFile(s.historyPath())
	if err != nil {
		return historyStore{}
	}
	var h historyStore
	if err := json.Unmarshal(data, &h); err != nil {
		return historyStore{}
	}
	return h
}

func (s *Server) listHistory(mode string) []HistoryEntry {
	s.historyMutex.Lock()
	defer s.historyMutex.Unlock()
	h := s.readHistory()
	if mode == "music" {
		return h.Music
	}
	return h.Files
}

func (s *Server) recordHistory(mode string, entry HistoryEntry) error {
	s.historyMutex.Lock()
	defer s.historyMutex.Unlock()
	h := s.readHistory()
	if mode == "music" {
		h.Music = prependHistory(h.Music, entry)
	} else {
		h.Files = prependHistory(h.Files, entry)
	}
	data, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return os.WriteFile(s.historyPath(), data, 0600)
}

func prependHistory(list []HistoryEntry, entry HistoryEntry) []HistoryEntry {
	list = slices.DeleteFunc(list, func(e HistoryEntry) bool { return e.Path == entry.Path })
	list = append([]HistoryEntry{entry}, list...)
	if len(list) > historyLimit {
		list = list[:historyLimit]
	}
	return list
}

func (s *Server) HandleHistory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		entries := s.listHistory(r.URL.Query().Get("mode"))
		if entries == nil {
			entries = []HistoryEntry{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)

	case http.MethodPost:
		var req struct {
			Mode string `json:"mode"`
			HistoryEntry
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		// Browse listings send "/a/b" and search results send "a/b" for the same
		// file, so dedup only holds once the two agree.
		req.Path = strings.TrimPrefix(req.Path, "/")
		if req.Name == "" || req.Path == "" {
			http.Error(w, "Missing name or path", http.StatusBadRequest)
			return
		}
		if err := s.recordHistory(req.Mode, req.HistoryEntry); err != nil {
			log.Printf("ERROR [server] failed to record history path=%s: %v", req.Path, err)
			http.Error(w, "Failed to record history", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
