package main

import (
	"html/template"
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

// --- Configuration ---
const (
	mediaRoot       = "media"
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

var appState = AppState{}

// --- Main Application Logic ---

func main() {
	setupDummyMediaDirectory()
	go scanMediaFilesPeriodically()

	mux := http.NewServeMux()

	// Core handlers
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/browse/", handleBrowseDirectory)
	mux.HandleFunc("/search", handleSearch)

	// Image viewer handlers
	mux.HandleFunc("/image-viewer/", handleImageViewer)
	mux.HandleFunc("/next-image", handleNextPrevImage)
	mux.HandleFunc("/prev-image", handleNextPrevImage)

	// Static file server for media
	fileServer := http.FileServer(http.Dir(mediaRoot))
	mux.Handle("/media/", http.StripPrefix("/media/", fileServer))

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

// --- HTTP Handlers ---

func serveIndex(w http.ResponseWriter, r *http.Request) {
	// This template now includes the full frontend structure from your reference document.
	const indexTemplate = `
<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Minimalist Media Viewer</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@1.0.4/css/bulma.min.css">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.2/css/all.min.css">
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <style>
        html { background-color: #222222; color: #eeeeee; }
        body { font-family: sans-serif; }
        .container { padding: 20px; }
        .box { background-color: #333333; color: #eeeeee; border: 1px solid #444444; }
        .card { background-color: #444; }
        .card-content p { color: #eee; }
        a, a:hover { color: #48c78e; }
        .title, .subtitle { color: #eeeeee; }
        .htmx-indicator { display: none; }
        .htmx-request .htmx-indicator { display: inline; }
        .htmx-request.htmx-indicator { display: inline; }
        .modal-background { background-color: rgba(0, 0, 0, 0.85); }
        .modal-content { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; overflow: hidden; }
        #modal-image { max-width: 95vw; max-height: 95vh; object-fit: contain; }
        .modal-close-custom { position: absolute; top: 20px; right: 20px; z-index: 1001; background: none; border: none; color: white; font-size: 2.5rem; cursor: pointer; }
        .image-viewer-controls { position: absolute; top: 50%; width: 100%; display: flex; justify-content: space-between; transform: translateY(-50%); padding: 0 10px; z-index: 1001; pointer-events: none; }
        .image-viewer-control-button { background: rgba(0,0,0,0.4); border: 1px solid #555; color: white; font-size: 2.5rem; cursor: pointer; padding: 10px 15px; border-radius: 8px; pointer-events: auto; }
        .image-viewer-control-button:hover { background: rgba(0,0,0,0.7); }
    </style>
</head>
<body>
    <section class="section">
        <div class="container">
            <!-- Search Bar -->
            <div class="field">
                <p class="control has-icons-left">
                    <input class="input is-dark" type="text" name="q" 
                           placeholder="Search all media..."
                           hx-get="/search" hx-trigger="keyup changed delay:300ms"
                           hx-target="#search-results-container" hx-indicator="#search-spinner">
                    <span class="icon is-small is-left"><i class="fas fa-search"></i></span>
                </p>
                <p id="search-spinner" class="help htmx-indicator">Searching...</p>
            </div>
            <div id="search-results-container"></div>

            <!-- Main Content Area -->
            <div id="main-content" hx-get="/browse/" hx-trigger="load" hx-swap="innerHTML">
                <progress class="progress is-small is-primary" max="100">Loading...</progress>
            </div>
        </div>
    </section>

    <!-- Image Viewer Modal -->
    <div id="image-modal" class="modal">
        <div class="modal-background" onclick="closeModal()"></div>
        <div class="modal-content">
            <div id="modal-image-container">
                <!-- HTMX will place the image content here -->
            </div>
        </div>
        <button class="modal-close-custom" aria-label="close" onclick="closeModal()">&times;</button>
    </div>

    <script>
        const imageModal = document.getElementById('image-modal');

        function closeModal() {
            imageModal.classList.remove('is-active');
        }

        document.addEventListener('keydown', (e) => {
            if (!imageModal.classList.contains('is-active')) return;
            if (e.key === 'Escape') {
                closeModal();
            } else if (e.key === 'ArrowLeft') {
                htmx.trigger('#prev-image-btn', 'click');
            } else if (e.key === 'ArrowRight') {
                htmx.trigger('#next-image-btn', 'click');
            }
        });

        let touchStartX = 0;
        document.addEventListener('touchstart', (e) => {
            if (!imageModal.classList.contains('is-active')) return;
            touchStartX = e.changedTouches[0].screenX;
        });

        document.addEventListener('touchend', (e) => {
            if (!imageModal.classList.contains('is-active')) return;
            const touchEndX = e.changedTouches[0].screenX;
            if (touchEndX < touchStartX - 50) { // Swiped left
                htmx.trigger('#next-image-btn', 'click');
            }
            if (touchEndX > touchStartX + 50) { // Swiped right
                htmx.trigger('#prev-image-btn', 'click');
            }
        });
    </script>
</body>
</html>`
	tmpl, _ := template.New("index").Parse(indexTemplate)
	tmpl.Execute(w, nil)
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
				info.ThumbURL = "/media/" + info.Path
				content.Images = append(content.Images, info)
			case ".mp4", ".webm", ".mkv":
				info.Type = "video"
				info.ThumbURL = "https://placehold.co/128x96/333333/eeeeee?text=Video"
				content.Videos = append(content.Videos, info)
			default:
				info.Type = "other"
				content.Others = append(content.Others, info)
			}
		}
	}

	const browseTemplate = `
<div class="box mt-4">
    <nav class="breadcrumb" aria-label="breadcrumbs">
        <ul>
            <li><a href="#" hx-get="/browse/" hx-target="#main-content"><span class="icon is-small"><i class="fas fa-home" aria-hidden="true"></i></span><span>Home</span></a></li>
            {{$current := .CurrentPath}}
            {{range $i, $part := split .CurrentPath "/"}}
                {{if $part}}
                    <li><a href="#" hx-get="/browse/{{join (slice (split $current "/") 0 (add $i 1)) "/"}}" hx-target="#main-content">{{$part}}</a></li>
                {{end}}
            {{end}}
        </ul>
    </nav>
    <hr class="m-0 mb-4">

    {{if .Folders}}
    <h3 class="subtitle is-5 has-text-grey-light">Folders</h3>
    <div class="list is-hoverable">
        {{range .Folders}}
        <a class="list-item" href="#" hx-get="/browse/{{.Path}}" hx-target="#main-content" hx-swap="innerHTML">
            <span class="icon"><i class="fas fa-folder"></i></span>&nbsp;{{.Name}}
        </a>
        {{end}}
    </div>
    {{end}}

    {{if .Images}}
    <h3 class="subtitle is-5 has-text-grey-light mt-5">Images</h3>
    <div class="columns is-multiline is-mobile">
        {{range .Images}}
        <div class="column is-one-quarter-desktop is-one-third-tablet is-half-mobile">
            <div class="card is-clickable" 
                 hx-get="/image-viewer/{{.Path}}" 
                 hx-target="#modal-image-container" 
                 hx-swap="innerHTML"
                 hx-on::after-swap="document.getElementById('image-modal').classList.add('is-active')">
                <div class="card-image">
                    <figure class="image is-4by3"><img src="{{.ThumbURL}}" alt="{{.Name}}" style="object-fit: cover;"></figure>
                </div>
                <div class="card-content p-2 has-text-centered"><p class="is-size-7 is-clipped">{{.Name}}</p></div>
            </div>
        </div>
        {{end}}
    </div>
    {{end}}

	{{if .Videos}}
    <h3 class="subtitle is-5 has-text-grey-light mt-5">Videos</h3>
    <div class="list">
        {{range .Videos}}
        <div class="list-item">
            <div class="list-item-content">
                <div class="list-item-title"><span class="icon"><i class="fas fa-film"></i></span>&nbsp;{{.Name}}</div>
            </div>
            <div class="list-item-controls"><a href="/media/{{.Path}}" target="_blank" class="button is-small is-light">Play</a></div>
        </div>
        {{end}}
    </div>
    {{end}}

	{{if .Others}}
    <h3 class="subtitle is-5 has-text-grey-light mt-5">Other Files</h3>
    <div class="list">
        {{range .Others}}
        <div class="list-item">
            <div class="list-item-content">
                <div class="list-item-title"><span class="icon"><i class="fas fa-file"></i></span>&nbsp;{{.Name}}</div>
            </div>
            <div class="list-item-controls"><a href="/media/{{.Path}}" target="_blank" class="button is-small is-light">Download</a></div>
        </div>
        {{end}}
    </div>
    {{end}}
</div>`

	funcMap := template.FuncMap{
		"split": strings.Split,
		"join":  strings.Join,
		"slice": func(s []string, start, end int) []string { return s[start:end] },
		"add":   func(a, b int) int { return a + b },
	}
	tmpl, _ := template.New("browse").Funcs(funcMap).Parse(browseTemplate)
	tmpl.Execute(w, content)
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("q"))
	if query == "" {
		w.Write([]byte(""))
		return
	}

	appState.mu.RLock()
	allFiles := appState.AllFiles
	appState.mu.RUnlock()

	var results []string
	for _, file := range allFiles {
		if strings.Contains(strings.ToLower(file), query) {
			results = append(results, file)
			if len(results) >= 20 {
				break
			}
		}
	}

	const searchTemplate = `
{{if .}}
<div class="box mt-4">
    <h3 class="subtitle is-5 has-text-light">Search Results</h3>
    <div class="list is-hoverable">
        {{range .}}
        <a class="list-item" href="#" hx-get="/browse/{{.Dir}}" hx-target="#main-content">
            <span class="icon is-small"><i class="fas fa-file-alt"></i></span>&nbsp;<span>{{.Path}}</span>
        </a>
        {{end}}
    </div>
</div>
{{end}}`
	type SearchResult struct{ Path, Dir string }
	var templateResults []SearchResult
	for _, res := range results {
		templateResults = append(templateResults, SearchResult{
			Path: res,
			Dir:  filepath.ToSlash(filepath.Dir(res)),
		})
	}

	tmpl, _ := template.New("search").Parse(searchTemplate)
	tmpl.Execute(w, templateResults)
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

	currentIndex := -1
	for i, img := range imagesInDir {
		if filepath.Join(dir, img) == currentImage {
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
	if err != nil {
		log.Printf("Could not get sorted images for %s: %v", dir, err)
		// Render just the image if we can't get next/prev
		tmpl, _ := template.New("image").Parse(imageViewerTemplate)
		tmpl.Execute(w, ImageViewerData{ImagePath: imagePath})
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

	tmpl, _ := template.New("image").Parse(imageViewerTemplate)
	tmpl.Execute(w, data)
}

const imageViewerTemplate = `
<img id="modal-image" src="/media/{{.ImagePath}}" alt="{{.ImagePath}}">
<div class="image-viewer-controls">
    <button id="prev-image-btn" class="image-viewer-control-button"
            hx-get="/prev-image?from={{.PrevPath}}"
            hx-target="#modal-image-container" hx-swap="innerHTML">&larr;</button>
    <button id="next-image-btn" class="image-viewer-control-button"
            hx-get="/next-image?from={{.NextPath}}"
            hx-target="#modal-image-container" hx-swap="innerHTML">&rarr;</button>
</div>`

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

func setupDummyMediaDirectory() {
	log.Println("Setting up dummy media directory...")
	os.RemoveAll(mediaRoot)

	dirs := []string{
		filepath.Join(mediaRoot, "Photos", "2024", "Vacation"),
		filepath.Join(mediaRoot, "Videos", "Movies"),
		filepath.Join(mediaRoot, "Documents"),
	}
	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
	}

	files := []struct{ path, content string }{
		{filepath.Join(dirs[0], "beach.jpg"), "fake jpg"},
		{filepath.Join(dirs[0], "sunset.png"), "fake png"},
		{filepath.Join(dirs[0], "mountains.webp"), "fake webp"},
		{filepath.Join(dirs[1], "epic_movie.mp4"), "fake mp4"},
		{filepath.Join(dirs[1], "short_clip.webm"), "fake webm"},
		{filepath.Join(dirs[2], "manual.pdf"), "fake pdf"},
		{filepath.Join(mediaRoot, "root_image.gif"), "fake gif"},
	}

	for _, file := range files {
		os.WriteFile(file.path, []byte(file.content), 0644)
	}
	log.Println("Dummy media setup complete.")
}
