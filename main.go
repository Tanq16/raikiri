// main.go
package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed frontend
var embeddedFrontend embed.FS

// --- Configuration ---
const (
	mediaRoot       = "Prox1" // Make sure you have a 'media' directory next to the executable
	serverPort      = ":8080"
	refreshInterval = 30 * time.Minute
)

// --- Data Structures ---

type FileInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type DirectoryContent struct {
	CurrentPath string     `json:"currentPath"`
	Breadcrumbs []FileInfo `json:"breadcrumbs"`
	Folders     []FileInfo `json:"folders"`
	Images      []FileInfo `json:"images"`
	Videos      []FileInfo `json:"videos"`
	Others      []FileInfo `json:"others"`
}

type AppState struct {
	mu       sync.RWMutex
	AllFiles []string
}

var appState = AppState{}

// --- Main Application Logic ---

func main() {
	// Check if media directory exists
	if _, err := os.Stat(mediaRoot); os.IsNotExist(err) {
		log.Printf("Creating media directory at './%s'", mediaRoot)
		os.Mkdir(mediaRoot, 0755)
	}

	go scanMediaFilesPeriodically()

	mux := http.NewServeMux()

	// Create a sub-filesystem for the 'frontend' directory
	frontendFS, err := fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	// Serve static files (index.html, styles.css, app.js) from the root of the sub-filesystem
	mux.Handle("/", http.FileServer(http.FS(frontendFS)))

	// API handlers
	mux.HandleFunc("/api/browse/", handleBrowse)
	mux.HandleFunc("/api/search", handleSearch)

	// Static file server for the actual media content
	mediaFileServer := http.FileServer(http.Dir(mediaRoot))
	mux.Handle("/media/", http.StripPrefix("/media/", mediaFileServer))

	log.Printf("Starting server on http://localhost%s", serverPort)
	log.Printf("Serving media from the '%s' directory.", mediaRoot)

	if err := http.ListenAndServe(serverPort, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// --- Background File Scanner ---

func scanMediaFilesPeriodically() {
	log.Println("Performing initial media scan...")
	scanAndCacheFiles()
	ticker := time.NewTicker(refreshInterval)
	for range ticker.C {
		log.Println("Refreshing media file list...")
		scanAndCacheFiles()
	}
}

func scanAndCacheFiles() {
	var fileList []string
	filepath.Walk(mediaRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relativePath, err := filepath.Rel(mediaRoot, path)
			if err == nil {
				fileList = append(fileList, filepath.ToSlash(relativePath))
			}
		}
		return nil
	})

	appState.mu.Lock()
	appState.AllFiles = fileList
	appState.mu.Unlock()
	log.Printf("Media scan complete. Found %d files.", len(fileList))
}

// --- API Handlers ---

func handleBrowse(w http.ResponseWriter, r *http.Request) {
	relativePath := strings.TrimPrefix(r.URL.Path, "/api/browse/")
	fullPath := filepath.Join(mediaRoot, relativePath)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, "Directory not found", http.StatusNotFound)
		return
	}

	content := DirectoryContent{
		CurrentPath: filepath.ToSlash(relativePath),
		Folders:     []FileInfo{},
		Images:      []FileInfo{},
		Videos:      []FileInfo{},
		Others:      []FileInfo{},
		Breadcrumbs: []FileInfo{},
	}

	// Build breadcrumbs
	content.Breadcrumbs = append(content.Breadcrumbs, FileInfo{Name: "Home", Path: ""})
	if relativePath != "" {
		parts := strings.Split(relativePath, "/")
		for i, part := range parts {
			content.Breadcrumbs = append(content.Breadcrumbs, FileInfo{
				Name: part,
				Path: strings.Join(parts[0:i+1], "/"),
			})
		}
	}

	for _, entry := range entries {
		info := FileInfo{
			Name: entry.Name(),
			Path: filepath.ToSlash(filepath.Join(relativePath, entry.Name())),
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))

		if entry.IsDir() {
			info.Type = "folder"
			content.Folders = append(content.Folders, info)
		} else {
			switch ext {
			case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
				info.Type = "image"
				content.Images = append(content.Images, info)
			case ".mp4", ".webm", ".mkv", ".mov", ".avi":
				info.Type = "video"
				content.Videos = append(content.Videos, info)
			default:
				info.Type = "other"
				content.Others = append(content.Others, info)
			}
		}
	}

	// Sort all slices alphabetically by name
	sort.Slice(content.Folders, func(i, j int) bool { return content.Folders[i].Name < content.Folders[j].Name })
	sort.Slice(content.Images, func(i, j int) bool { return content.Images[i].Name < content.Images[j].Name })
	sort.Slice(content.Videos, func(i, j int) bool { return content.Videos[i].Name < content.Videos[j].Name })
	sort.Slice(content.Others, func(i, j int) bool { return content.Others[i].Name < content.Others[j].Name })

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(content)
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("q"))
	if query == "" {
		json.NewEncoder(w).Encode([]FileInfo{})
		return
	}

	appState.mu.RLock()
	allFiles := appState.AllFiles
	appState.mu.RUnlock()

	var results []FileInfo
	for _, file := range allFiles {
		if strings.Contains(strings.ToLower(file), query) {
			results = append(results, FileInfo{
				Name: filepath.Base(file),
				Path: file,
				Type: "file", // Generic type for search results
			})
			if len(results) >= 50 { // Limit results
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
