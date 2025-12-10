package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed public
var staticFiles embed.FS

var (
	mediaPath     string
	musicPath     string
	cachePath     string
	activeStreams = make(map[string]*exec.Cmd)
	streamMutex   sync.Mutex
)

type FileEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"` // Relative path from root of mode
	Type     string `json:"type"` // folder, audio, video, image, other
	Size     string `json:"size"`
	Thumb    string `json:"thumb,omitempty"`
	Modified string `json:"modified,omitempty"`
}

func main() {
	prepare := flag.Bool("prepare", false, "Generate thumbnails for all video files and exit")
	flag.StringVar(&mediaPath, "media", ".", "Path to media directory")
	flag.StringVar(&musicPath, "music", "./music", "Path to music directory")
	flag.StringVar(&cachePath, "cache", "/tmp", "Path to cache directory for HLS segments")
	flag.Parse()

	if *prepare {
		log.Println("Starting in prepare mode. Generating thumbnails for video files...")
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			log.Fatalf("Error: `ffmpeg` is not installed or not in your system's PATH. Please install ffmpeg to use the prepare feature.")
		}
		if _, err := exec.LookPath("ffprobe"); err != nil {
			log.Fatalf("Error: `ffprobe` is not installed or not in your system's PATH. Please install ffmpeg (which includes ffprobe) to use the prepare feature.")
		}
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Error getting current working directory: %v", err)
		}
		processDirectoryForThumbnails(cwd)
		log.Println("Thumbnail generation complete.")
		return
	}

	// Clean up and create cache directory
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		log.Fatalf("Failed to create cache directory: %v", err)
	}

	http.HandleFunc("/api/list", handleList)
	http.HandleFunc("/api/stream", handleStreamStart)
	http.HandleFunc("/api/stop-stream", handleStreamStop)
	http.HandleFunc("/api/upload", handleUpload)
	http.HandleFunc("/content/", handleContent)

	hlsHandler := makeHLSHandler(cachePath)
	http.Handle("/hls/", http.StripPrefix("/hls/", logRequests("hls", hlsHandler)))
	http.Handle("/api/hls/", http.StripPrefix("/api/hls/", logRequests("api/hls", hlsHandler)))

	http.Handle("/", http.FileServer(http.FS(mustSub(staticFiles, "public"))))

	fmt.Printf("Raikiri running on :8080\nMedia: %s\nMusic: %s\nCache: %s\n", mediaPath, musicPath, cachePath)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func mustSub(f embed.FS, path string) fs.FS {
	sub, err := fs.Sub(f, path)
	if err != nil {
		panic(err)
	}
	return sub
}

func getRoot(mode string) string {
	if mode == "music" {
		return musicPath
	}
	return mediaPath
}

func getFileType(name string, isDir bool) string {
	if isDir {
		return "folder"
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".mp3", ".flac", ".wav", ".m4a", ".ogg":
		return "audio"
	case ".mp4", ".mkv", ".webm", ".mov", ".avi":
		return "video"
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return "image"
	case ".pdf":
		return "pdf"
	case ".txt", ".md":
		return "text"
	}
	return "file"
}

func handleContent(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	relPath := strings.TrimPrefix(r.URL.Path, "/content/")
	fullPath := filepath.Join(getRoot(mode), relPath)
	http.ServeFile(w, r, fullPath)
}

func handleList(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	relPath := r.URL.Query().Get("path")
	recursive := r.URL.Query().Get("recursive") == "true"

	root := getRoot(mode)
	targetDir := filepath.Join(root, relPath)

	var entries []FileEntry

	if recursive {
		err := filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if rel == "." {
				return nil
			}

			if d.IsDir() {
				thumbPath := filepath.ToSlash(filepath.Join(rel, ".thumbnail.jpg"))
				entries = append(entries, FileEntry{
					Name:     name,
					Path:     rel,
					Type:     "folder",
					Size:     "",
					Thumb:    thumbPath,
					Modified: info.ModTime().Format("2006-01-02 15:04"),
				})
				return nil
			}

			fType := getFileType(name, false)
			if fType == "audio" || fType == "video" || fType == "image" {
				size := fmt.Sprintf("%.1f MB", float64(info.Size())/1024/1024)

				thumbPath := ""
				if fType == "video" || fType == "image" || fType == "audio" {
					if mode == "music" && fType == "audio" {
						dir := filepath.Dir(rel)
						thumbPath = filepath.Join(dir, ".thumbnail.jpg")
					} else {
						thumbPath = filepath.Join(rel, "."+name+".thumbnail.jpg")
					}
					thumbPath = filepath.ToSlash(thumbPath)
				}

				entries = append(entries, FileEntry{
					Name:     name,
					Path:     rel,
					Type:     fType,
					Size:     size,
					Thumb:    thumbPath,
					Modified: info.ModTime().Format("2006-01-02 15:04"),
				})
			}
			return nil
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	} else {
		files, err := os.ReadDir(targetDir)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		for _, f := range files {
			if strings.HasPrefix(f.Name(), ".") {
				continue
			}

			info, err := f.Info()
			if err != nil {
				continue // Skip files we can't stat
			}
			size := ""
			if !f.IsDir() {
				size = fmt.Sprintf("%.1f MB", float64(info.Size())/1024/1024)
			}
			modified := info.ModTime().Format("2006-01-02 15:04")

			fType := getFileType(f.Name(), f.IsDir())

			// Generate relative path from ROOT, not from current folder
			fullRelPath := filepath.Join(relPath, f.Name())
			fullRelPath = filepath.ToSlash(fullRelPath)

			// Determine thumbnail path logic
			thumbPath := ""
			if f.IsDir() {
				thumbPath = filepath.Join(relPath, f.Name(), ".thumbnail.jpg")
			} else if fType == "video" || fType == "image" || fType == "audio" {
				if mode == "music" && fType == "audio" {
					thumbPath = filepath.Join(relPath, ".thumbnail.jpg")
				} else {
					thumbPath = filepath.Join(relPath, "."+f.Name()+".thumbnail.jpg")
				}
			}
			thumbPath = filepath.ToSlash(thumbPath)

			entries = append(entries, FileEntry{
				Name:     f.Name(),
				Path:     fullRelPath,
				Type:     fType,
				Size:     size,
				Thumb:    thumbPath,
				Modified: modified,
			})
		}
	}

	// Sort: Folders first, then alphabetically
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type == "folder" && entries[j].Type != "folder" {
			return true
		}
		if entries[i].Type != "folder" && entries[j].Type == "folder" {
			return false
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mode := r.FormValue("mode")
	relPath := r.FormValue("path")

	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	files := r.MultipartForm.File["files"]
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		dstPath := filepath.Join(getRoot(mode), relPath, fileHeader.Filename)
		dst, err := os.Create(dstPath)
		if err != nil {
			file.Close()
			http.Error(w, err.Error(), 500)
			return
		}

		if _, err := io.Copy(dst, file); err != nil {
			file.Close()
			dst.Close()
			http.Error(w, err.Error(), 500)
			return
		}

		file.Close()
		dst.Close()
	}

	w.WriteHeader(200)
}

// generateHLSPlaylist creates a VOD m3u8 playlist file upfront based on video duration
func generateHLSPlaylist(playlistPath string, duration float64, segmentDuration float64) error {
	numSegments := int(duration / segmentDuration)
	lastSegmentDuration := duration - (float64(numSegments) * segmentDuration)

	// If there's a remainder, we need one more segment
	if lastSegmentDuration > 0 {
		numSegments++
	} else {
		lastSegmentDuration = segmentDuration
	}

	// Build the m3u8 content
	var content strings.Builder
	content.WriteString("#EXTM3U\n")
	content.WriteString("#EXT-X-VERSION:3\n")
	content.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", int(segmentDuration)+1))
	content.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	content.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")

	// Write all segment entries
	for i := 0; i < numSegments; i++ {
		segDur := segmentDuration
		if i == numSegments-1 {
			segDur = lastSegmentDuration
		}
		content.WriteString(fmt.Sprintf("#EXTINF:%.6f,\n", segDur))
		content.WriteString(fmt.Sprintf("seg_%03d.ts\n", i))
	}

	content.WriteString("#EXT-X-ENDLIST\n")

	// Write the playlist file
	return os.WriteFile(playlistPath, []byte(content.String()), 0644)
}

func handleStreamStart(w http.ResponseWriter, r *http.Request) {
	targetFile := r.URL.Query().Get("file")
	mode := r.URL.Query().Get("mode")
	root := getRoot(mode)
	fullPath := filepath.Join(root, targetFile)

	duration, err := getVideoDuration(fullPath)
	if err != nil {
		http.Error(w, "Failed to get video duration", 500)
		return
	}

	audioCodec := getAudioCodec(fullPath)
	var audioArgs []string
	if isAudioCompatible(audioCodec) {
		audioArgs = []string{"-c:a", "copy"}
	} else {
		audioArgs = []string{"-c:a", "aac", "-b:a", "128k"}
	}

	sessionID := fmt.Sprintf("s_%d", time.Now().UnixNano())
	sessionDir := filepath.Join(cachePath, sessionID)
	os.MkdirAll(sessionDir, 0755)

	playlistPath := filepath.Join(sessionDir, "index.m3u8")
	segmentPath := filepath.Join(sessionDir, "seg_%03d.ts")

	// Pre-generate the m3u8 playlist so it's available immediately
	const segmentDuration = 6.0
	if err := generateHLSPlaylist(playlistPath, duration, segmentDuration); err != nil {
		log.Printf("Failed to generate playlist: %v", err)
		http.Error(w, "Failed to generate playlist", 500)
		return
	}
	log.Printf("Pre-generated HLS playlist: session=%s duration=%.2fs segments=%d", sessionID, duration, int(duration/segmentDuration)+1)

	args := []string{
		"-i", fullPath,
		"-c:v", "copy",
	}
	args = append(args, audioArgs...)
	args = append(args,
		"-f", "hls",
		"-hls_time", "6",
		"-hls_list_size", "0", // 0 = unlimited, keep all segments in playlist
		"-hls_playlist_type", "vod", // VOD playlist type for full seekability
		"-hls_segment_filename", segmentPath,
		playlistPath,
	)

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start ffmpeg: %v", err)
		http.Error(w, "Failed to start stream", 500)
		return
	}

	log.Printf("Started HLS stream: session=%s file=%s", sessionID, targetFile)

	streamMutex.Lock()
	activeStreams[sessionID] = cmd
	streamMutex.Unlock()

	// Wait until first segment exists (playlist is already pre-generated)
	firstSeg := filepath.Join(sessionDir, "seg_000.ts")
	firstSegReady := waitForFile(firstSeg, 50, 200*time.Millisecond)
	if !firstSegReady {
		log.Printf("HLS not ready: first segment not available")
		http.Error(w, "Stream not ready", http.StatusServiceUnavailable)
		return
	}

	log.Printf("HLS ready: session=%s (playlist pre-generated, first segment available)", sessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		// Prefer the /api prefix so setups that only proxy /api still reach us, but keep /hls for direct access.
		"url":       fmt.Sprintf("/api/hls/%s/index.m3u8", sessionID),
		"altUrl":    fmt.Sprintf("/hls/%s/index.m3u8", sessionID),
		"sessionId": sessionID,
		"duration":  duration,
	})
}

func handleStreamStop(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		return
	}

	streamMutex.Lock()
	if cmd, exists := activeStreams[sessionID]; exists {
		cmd.Process.Kill()
		cmd.Wait()
		delete(activeStreams, sessionID)
		log.Printf("Stopped HLS stream: session=%s", sessionID)
		go os.RemoveAll(filepath.Join(cachePath, sessionID))
	}
	streamMutex.Unlock()

	w.WriteHeader(200)
}

// Minimal request logger used for HLS so we can see if requests reach the server.
func logRequests(prefix string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("HLS request [%s]: %s", prefix, r.URL.Path)
		h.ServeHTTP(w, r)
	})
}

// makeHLSHandler serves files from the HLS temp directory with explicit existence checks
// and path cleaning to avoid traversal and to log misses.
func makeHLSHandler(root string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean and ensure the path stays within root.
		rel := strings.TrimPrefix(r.URL.Path, "/")
		rel = filepath.Clean(rel)
		fullPath := filepath.Join(root, rel)
		if !strings.HasPrefix(fullPath, root) {
			log.Printf("HLS rejected traversal: %s", fullPath)
			http.NotFound(w, r)
			return
		}
		if _, err := os.Stat(fullPath); err != nil {
			log.Printf("HLS miss: %s (%v)", fullPath, err)
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, fullPath)
	})
}

// waitForFile waits for a file to exist and be non-empty for up to attempts*sleep.
func waitForFile(path string, attempts int, sleep time.Duration) bool {
	for i := 0; i < attempts; i++ {
		info, err := os.Stat(path)
		if err == nil && info.Size() > 0 {
			return true
		}
		time.Sleep(sleep)
	}
	return false
}

// Thumbnail Generation

func getVideoDuration(filePath string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", filePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get video duration: %w", err)
	}
	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}
	return duration, nil
}

func getAudioCodec(filePath string) string {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "a:0", "-show_entries", "stream=codec_name", "-of", "default=noprint_wrappers=1:nokey=1", filePath)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func isAudioCompatible(codec string) bool {
	compatible := []string{"aac", "mp3", "ac3", "eac3", "opus"}
	for _, c := range compatible {
		if codec == c {
			return true
		}
	}
	return false
}

func formatDuration(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := int(seconds) % 3600 / 60
	secs := int(seconds) % 60
	frac := seconds - float64(int(seconds))
	millis := int(frac * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, millis)
}

func createVideoThumbnail(filePath string) error {
	dir := filepath.Dir(filePath)
	filename := filepath.Base(filePath)
	thumbFilename := fmt.Sprintf(".%s.thumbnail.jpg", filename)
	thumbPath := filepath.Join(dir, thumbFilename)

	// Skip if thumbnail already exists
	if _, err := os.Stat(thumbPath); err == nil {
		return nil
	}

	// Get video duration
	duration, err := getVideoDuration(filePath)
	if err != nil {
		return fmt.Errorf("failed to get video duration: %w", err)
	}

	// Calculate 50% of duration, but ensure it doesn't exceed the actual duration
	seekTime := duration / 2.0
	if seekTime >= duration {
		seekTime = duration - 0.5 // Seek to 0.5 seconds before end if duration is very short
		if seekTime < 0 {
			seekTime = 0
		}
	}
	seekTimeStr := formatDuration(seekTime)

	// Create thumbnail at 50% of video duration with -ss before -i for fast input seeking
	cmd := exec.Command("ffmpeg", "-ss", seekTimeStr, "-i", filePath, "-vframes", "1", "-vf", "scale=400:-1", "-q:v", "3", "-y", thumbPath)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create thumbnail for %s: %w", filename, err)
	}
	return nil
}

func isVideoFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	videoExts := []string{".mp4", ".mkv", ".webm", ".mov", ".avi"}
	for _, ve := range videoExts {
		if ext == ve {
			return true
		}
	}
	return false
}

func processDirectoryForThumbnails(rootDir string) {
	var filesToProcess []string
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasPrefix(info.Name(), ".") {
			if isVideoFile(info.Name()) {
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
	log.Printf("Found %d video files to process in '%s'.", totalFiles, rootDir)
	for i, filePath := range filesToProcess {
		err := createVideoThumbnail(filePath)
		if err != nil {
			fmt.Printf("\nERROR: %s - %v\n", filePath, err)
		} else {
			fmt.Printf("\r%d / %d files done", i+1, totalFiles)
		}
	}
	fmt.Println()
}
