package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed frontend
var embeddedFrontend embed.FS

var imageExtensions []string = []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
var videoExtensions []string = []string{".mp4", ".webm", ".mov", ".avi"}
var audioExtensions []string = []string{".mp3", ".wav", ".m4a"} //, ".ogg", ".flac"}
var textExtensions []string = []string{".txt", ".md", ".log"}

type FileInfo struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	Type          string `json:"type"`
	ThumbnailPath string `json:"thumbnailPath,omitempty"`
}

type DirectoryContent struct {
	CurrentPath string     `json:"currentPath"`
	Breadcrumbs []FileInfo `json:"breadcrumbs"`
	Folders     []FileInfo `json:"folders"`
	Images      []FileInfo `json:"images"`
	Videos      []FileInfo `json:"videos"`
	Audios      []FileInfo `json:"audios"`
	Others      []FileInfo `json:"others"`
}

type AppState struct {
	mu        sync.RWMutex
	AllFiles  []string
	MediaRoot string
}

var appState = AppState{}

func main() {
	mediaDir := flag.String("media", "./media", "Path to the media directory")
	port := flag.String("port", "8080", "Port to run the server on")
	refreshMinutes := flag.Int("refresh", 30, "Interval in minutes to refresh the file listing")
	prepare := flag.Bool("prepare", false, "Generate thumbnails for all media and exit")
	forced := flag.Bool("force", false, "Force re-update thumbnails")
	flag.Parse()
	appState.MediaRoot = *mediaDir
	if _, err := os.Stat(appState.MediaRoot); os.IsNotExist(err) {
		log.Printf("Creating media directory at '%s'", appState.MediaRoot)
		os.MkdirAll(appState.MediaRoot, 0755)
	}

	if *prepare {
		log.Println("Starting in prepare mode. Generating thumbnails...")
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			log.Fatalf("Error: `ffmpeg` is not installed or not in your system's PATH. Please install ffmpeg to use the prepare feature.")
		}
		processDirectoryForThumbnails(appState.MediaRoot, *forced)
		log.Println("Thumbnail generation complete.")
		return
	}

	go scanMediaFilesPeriodically(time.Duration(*refreshMinutes) * time.Minute)
	mux := http.NewServeMux()
	frontendFS, err := fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	mux.Handle("/", http.FileServer(http.FS(frontendFS)))
	mux.HandleFunc("/api/sync", handleSync)
	mux.HandleFunc("/api/browse/", handleBrowse)
	mux.HandleFunc("/api/search", handleSearch)
	mediaFileServer := http.FileServer(http.Dir(appState.MediaRoot))
	mux.Handle("/media/", http.StripPrefix("/media/", mediaFileServer))
	serverPort := ":" + *port
	log.Printf("Starting server on http://localhost%s", serverPort)
	log.Printf("Serving media from the '%s' directory.", appState.MediaRoot)
	if err := http.ListenAndServe(serverPort, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// Thumbnail Generation

func createThumbnail(filePath string, forced bool) error {
	dir := filepath.Dir(filePath)
	filename := filepath.Base(filePath)
	thumbFilename := fmt.Sprintf(".%s.raithumb.jpg", filename)
	thumbPath := filepath.Join(dir, thumbFilename)
	size := "320x180"
	// Skip if thumbnail exists, unless forced
	if !forced {
		if _, err := os.Stat(thumbPath); err == nil {
			return nil
		}
	}
	var cmd *exec.Cmd
	ext := strings.ToLower(filepath.Ext(filePath))
	isImage := slices.Contains(imageExtensions, ext)
	isVideo := slices.Contains(videoExtensions, ext)
	if isImage {
		cmd = exec.Command("ffmpeg", "-i", filePath, "-vf", fmt.Sprintf("scale=%s:-1", strings.Split(size, "x")[0]), "-q:v", "3", "-y", thumbPath)
	} else if isVideo {
		cmd = exec.Command("ffmpeg", "-i", filePath, "-ss", "00:00:30", "-vframes", "1", "-vf", fmt.Sprintf("scale=%s:-1", strings.Split(size, "x")[0]), "-q:v", "3", "-y", thumbPath)
	} else {
		return nil // file not supported
	}
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create thumbnail for %s: %w", filename, err)
	}
	return nil
}

func processDirectoryForThumbnails(rootDir string, forced bool) {
	supportedExtensions := []string{}
	supportedExtensions = append(supportedExtensions, imageExtensions...)
	supportedExtensions = append(supportedExtensions, videoExtensions...)
	var filesToProcess []string
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasPrefix(info.Name(), ".") {
			ext := strings.ToLower(filepath.Ext(info.Name()))
			if slices.Contains(supportedExtensions, ext) {
				filesToProcess = append(filesToProcess, path)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error walking directory: %v", err)
		return
	}
	totalFiles := len(filesToProcess)
	log.Printf("Found %d media files to process in '%s'.", totalFiles, rootDir)
	for i, filePath := range filesToProcess {
		err := createThumbnail(filePath, forced)
		if err != nil {
			fmt.Printf("\nERROR: %s\n", filePath)
		}
		fmt.Printf("\r%d / %d files done", i+1, totalFiles)
	}
	fmt.Println()
}

// Background State Updater

func scanMediaFilesPeriodically(interval time.Duration) {
	log.Println("Performing initial media scan...")
	scanAndCacheFiles()
	ticker := time.NewTicker(interval)
	for range ticker.C {
		log.Println("Refreshing media file list...")
		scanAndCacheFiles()
	}
}

func scanAndCacheFiles() {
	var fileList []string
	filepath.Walk(appState.MediaRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasPrefix(info.Name(), ".") {
			relativePath, err := filepath.Rel(appState.MediaRoot, path)
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

// API Handlers

func handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	log.Println("Manual sync triggered via API.")
	scanAndCacheFiles()
	response := map[string]string{"status": "sync_completed"}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func handleBrowse(w http.ResponseWriter, r *http.Request) {
	relativePath := strings.TrimPrefix(r.URL.Path, "/api/browse/")
	fullPath := filepath.Join(appState.MediaRoot, relativePath)
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
		Audios:      []FileInfo{},
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
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info := FileInfo{
			Name: entry.Name(),
			Path: filepath.ToSlash(filepath.Join(relativePath, entry.Name())),
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if entry.IsDir() {
			info.Type = "folder"
			content.Folders = append(content.Folders, info)
		} else {
			// Check for thumbnail
			thumbFilename := fmt.Sprintf(".%s.raithumb.jpg", entry.Name())
			thumbPhysicalPath := filepath.Join(fullPath, thumbFilename)
			if _, err := os.Stat(thumbPhysicalPath); err == nil {
				info.ThumbnailPath = filepath.ToSlash(filepath.Join(relativePath, thumbFilename))
			}

			if slices.Contains(imageExtensions, ext) {
				info.Type = "image"
				content.Images = append(content.Images, info)
			} else if slices.Contains(videoExtensions, ext) {
				info.Type = "video"
				content.Videos = append(content.Videos, info)
			} else if slices.Contains(audioExtensions, ext) {
				info.Type = "audio"
				content.Audios = append(content.Audios, info)
			} else {
				if ext == ".pdf" {
					info.Type = "pdf"
				} else if slices.Contains(textExtensions, ext) {
					info.Type = "text"
				} else {
					info.Type = "other"
				}
				content.Others = append(content.Others, info)
			}
		}
	}
	sort.Slice(content.Folders, func(i, j int) bool { return content.Folders[i].Name < content.Folders[j].Name })
	sort.Slice(content.Images, func(i, j int) bool { return content.Images[i].Name < content.Images[j].Name })
	sort.Slice(content.Videos, func(i, j int) bool { return content.Videos[i].Name < content.Videos[j].Name })
	sort.Slice(content.Audios, func(i, j int) bool { return content.Audios[i].Name < content.Audios[j].Name })
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
	searchWords := strings.Fields(query)
	if len(searchWords) == 0 {
		json.NewEncoder(w).Encode([]FileInfo{})
		return
	}
	appState.mu.RLock()
	allFiles := appState.AllFiles
	appState.mu.RUnlock()
	var results []FileInfo
	for _, file := range allFiles {
		lowerFile := strings.ToLower(file)
		matchesAll := true
		// fuzzy search all words as substrings of path
		for _, word := range searchWords {
			if !strings.Contains(lowerFile, word) {
				matchesAll = false
				break
			}
		}
		if matchesAll {
			fileInfo := FileInfo{
				Name: filepath.Base(file),
				Path: file,
				Type: "file",
			}
			// Check for thumbnail
			dir := filepath.Dir(filepath.Join(appState.MediaRoot, file))
			filename := filepath.Base(file)
			thumbFilename := fmt.Sprintf(".%s.raithumb.jpg", filename)
			thumbPhysicalPath := filepath.Join(dir, thumbFilename)
			if _, err := os.Stat(thumbPhysicalPath); err == nil {
				thumbRelativePath := filepath.Join(filepath.Dir(file), thumbFilename)
				fileInfo.ThumbnailPath = filepath.ToSlash(thumbRelativePath)
			}
			results = append(results, fileInfo)
			if len(results) >= 50 { // Limit results
				break
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
