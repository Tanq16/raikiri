package main

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
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
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	ThumbURL string `json:"thumbURL"`
}

type DirectoryContent struct {
	CurrentPath string
	ParentPath  string
	Folders     []FileInfo
	Images      []FileInfo
	Videos      []FileInfo
	Others      []FileInfo
}

type ImageViewerData struct {
	ImagePath string
	PrevPath  string
	NextPath  string
}

type AppState struct {
	mu       sync.RWMutex
	AllFiles []string
}

var (
	appState  = AppState{}
	templates *template.Template
)

// --- Main Application Logic ---

func main() {
	// Check if media directory exists
	if _, err := os.Stat(mediaRoot); os.IsNotExist(err) {
		log.Printf("Creating media directory at './%s'", mediaRoot)
		os.Mkdir(mediaRoot, 0755)
	}

	// Parse all templates from the embedded filesystem
	var err error
	templates, err = parseTemplates()
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	go scanMediaFilesPeriodically()

	mux := http.NewServeMux()

	// Serve static files (like app.js) from the embedded FS
	staticFS, err := fs.Sub(embeddedFrontend, "frontend/static")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Core handlers
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/browse/", handleBrowseDirectory)
	mux.HandleFunc("/search", handleSearch)

	// Image viewer handlers
	mux.HandleFunc("/image-viewer/", handleImageViewer)
	mux.HandleFunc("/next-image", handleNextPrevImage)
	mux.HandleFunc("/prev-image", handleNextPrevImage)

	// Static file server for the actual media content
	mediaFileServer := http.FileServer(http.Dir(mediaRoot))
	mux.Handle("/media/", http.StripPrefix("/media/", mediaFileServer))

	log.Printf("Starting server on http://localhost%s", serverPort)
	log.Printf("Serving media from the '%s' directory.", mediaRoot)

	if err := http.ListenAndServe(serverPort, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// --- Template Parsing ---
func parseTemplates() (*template.Template, error) {
	// Custom functions to be used in templates
	funcMap := template.FuncMap{
		"split": strings.Split,
		"join":  strings.Join,
		"slice": func(s []string, start, end int) []string { return s[start:end] },
		"add":   func(a, b int) int { return a + b },
		"dir":   func(path string) string { return filepath.ToSlash(filepath.Dir(path)) },
	}

	// Parse all .html files from the templates directory
	return template.New("").Funcs(funcMap).ParseFS(embeddedFrontend, "frontend/templates/*.html")
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

// --- HTTP Handlers ---

func serveIndex(w http.ResponseWriter, r *http.Request) {
	// The base template will trigger htmx to load the initial directory view
	err := templates.ExecuteTemplate(w, "base.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleBrowseDirectory(w http.ResponseWriter, r *http.Request) {
	relativePath := strings.TrimPrefix(r.URL.Path, "/browse/")
	fullPath := filepath.Join(mediaRoot, relativePath)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, "Directory not found", http.StatusNotFound)
		return
	}

	content := DirectoryContent{CurrentPath: filepath.ToSlash(relativePath)}
	if relativePath != "" && relativePath != "." {
		content.ParentPath = filepath.ToSlash(filepath.Dir(relativePath))
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
			case ".jpg", ".jpeg", ".png", ".gif", ".webp":
				info.Type = "image"
				info.ThumbURL = "/media/" + info.Path // For simplicity, using full image as thumb
				content.Images = append(content.Images, info)
			case ".mp4", ".webm", ".mkv":
				info.Type = "video"
				// Placeholder for video thumbnail
				info.ThumbURL = "https://placehold.co/128x96/333333/eeeeee?text=Video"
				content.Videos = append(content.Videos, info)
			default:
				info.Type = "other"
				content.Others = append(content.Others, info)
			}
		}
	}

	err = templates.ExecuteTemplate(w, "browse.html", content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("q"))
	if query == "" {
		w.Write([]byte("")) // Return empty response to clear results
		return
	}

	appState.mu.RLock()
	allFiles := appState.AllFiles
	appState.mu.RUnlock()

	var results []string
	for _, file := range allFiles {
		if strings.Contains(strings.ToLower(file), query) {
			results = append(results, file)
			if len(results) >= 20 { // Limit results
				break
			}
		}
	}

	err := templates.ExecuteTemplate(w, "search_results.html", results)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleImageViewer(w http.ResponseWriter, r *http.Request) {
	imagePath := strings.TrimPrefix(r.URL.Path, "/image-viewer/")
	renderImageViewer(w, imagePath)
}

func handleNextPrevImage(w http.ResponseWriter, r *http.Request) {
	currentImage := r.URL.Query().Get("from")
	if currentImage == "" {
		http.Error(w, "Missing 'from' parameter", http.StatusBadRequest)
		return
	}

	dir := filepath.Dir(currentImage)
	imagesInDir, err := getSortedImagesInDir(filepath.Join(mediaRoot, dir))
	if err != nil {
		http.Error(w, "Could not read directory", http.StatusInternalServerError)
		return
	}

	if len(imagesInDir) == 0 {
		renderImageViewer(w, currentImage) // Render self if no other images
		return
	}

	currentIndex := -1
	for i, img := range imagesInDir {
		if filepath.ToSlash(filepath.Join(dir, img)) == currentImage {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		http.Error(w, "Image not found in directory", http.StatusNotFound)
		return
	}

	var nextIndex int
	if strings.Contains(r.URL.Path, "/next-image") {
		nextIndex = (currentIndex + 1) % len(imagesInDir)
	} else { // prev-image
		nextIndex = (currentIndex - 1 + len(imagesInDir)) % len(imagesInDir)
	}

	nextImagePath := filepath.ToSlash(filepath.Join(dir, imagesInDir[nextIndex]))
	renderImageViewer(w, nextImagePath)
}

func renderImageViewer(w http.ResponseWriter, imagePath string) {
	dir := filepath.Dir(imagePath)
	imagesInDir, err := getSortedImagesInDir(filepath.Join(mediaRoot, dir))
	if err != nil || len(imagesInDir) < 2 { // Can't get prev/next if error or only 1 image
		data := ImageViewerData{ImagePath: imagePath}
		templates.ExecuteTemplate(w, "image_viewer.html", data)
		return
	}

	currentIndex := -1
	for i, img := range imagesInDir {
		if filepath.ToSlash(filepath.Join(dir, img)) == imagePath {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		http.Error(w, "Could not find image index", http.StatusInternalServerError)
		return
	}

	prevIndex := (currentIndex - 1 + len(imagesInDir)) % len(imagesInDir)
	nextIndex := (currentIndex + 1) % len(imagesInDir)

	data := ImageViewerData{
		ImagePath: imagePath,
		PrevPath:  url.QueryEscape(filepath.ToSlash(filepath.Join(dir, imagesInDir[prevIndex]))),
		NextPath:  url.QueryEscape(filepath.ToSlash(filepath.Join(dir, imagesInDir[nextIndex]))),
	}

	err = templates.ExecuteTemplate(w, "image_viewer.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// --- Utility Functions ---

func getSortedImagesInDir(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	var imageNames []string
	for _, entry := range entries {
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" {
				imageNames = append(imageNames, entry.Name())
			}
		}
	}
	sort.Strings(imageNames) // Sort alphabetically
	return imageNames, nil
}
