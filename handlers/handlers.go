package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	MediaPath     string
	MusicPath     string
	CachePath     string
	ActiveStreams = make(map[string]*exec.Cmd)
	StreamMutex   sync.Mutex
)

type FileEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"` // Relative path from root of mode
	Type     string `json:"type"` // folder, audio, video, image, other
	Size     string `json:"size"`
	Thumb    string `json:"thumb,omitempty"`
	Modified string `json:"modified,omitempty"`
}

func GetRoot(mode string) string {
	if mode == "music" {
		return MusicPath
	}
	return MediaPath
}

func GetFileType(name string, isDir bool) string {
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

func GetThumbnailPath(relPath, fileName, fileType, mode string) string {
	if fileType == "folder" {
		return filepath.ToSlash(filepath.Join(relPath, ".thumbnail.jpg"))
	}
	if mode == "music" && fileType == "audio" {
		return filepath.ToSlash(filepath.Join(relPath, ".thumbnail.jpg"))
	}
	return filepath.ToSlash(filepath.Join(relPath, "."+fileName+".thumbnail.jpg"))
}

func FormatFileSize(bytes int64) string {
	return fmt.Sprintf("%.1f MB", float64(bytes)/1024/1024)
}

func FormatModTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

func HandleContent(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	relPath := strings.TrimPrefix(r.URL.Path, "/content/")
	fullPath := filepath.Join(GetRoot(mode), relPath)
	http.ServeFile(w, r, fullPath)
}

func HandleList(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	relPath := r.URL.Query().Get("path")
	recursive := r.URL.Query().Get("recursive") == "true"

	root := GetRoot(mode)
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
				entries = append(entries, FileEntry{
					Name:     name,
					Path:     rel,
					Type:     "folder",
					Size:     "",
					Thumb:    GetThumbnailPath(rel, name, "folder", mode),
					Modified: FormatModTime(info.ModTime()),
				})
				return nil
			}

			fType := GetFileType(name, false)
			if fType == "audio" || fType == "video" || fType == "image" {
				dir := filepath.Dir(rel)

				entries = append(entries, FileEntry{
					Name:     name,
					Path:     rel,
					Type:     fType,
					Size:     FormatFileSize(info.Size()),
					Thumb:    GetThumbnailPath(dir, name, fType, mode),
					Modified: FormatModTime(info.ModTime()),
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
				size = FormatFileSize(info.Size())
			}

			fType := GetFileType(f.Name(), f.IsDir())

			// Generate relative path from ROOT, not from current folder
			fullRelPath := filepath.Join(relPath, f.Name())
			fullRelPath = filepath.ToSlash(fullRelPath)

			entries = append(entries, FileEntry{
				Name:     f.Name(),
				Path:     fullRelPath,
				Type:     fType,
				Size:     size,
				Thumb:    GetThumbnailPath(relPath, f.Name(), fType, mode),
				Modified: FormatModTime(info.ModTime()),
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

func HandleUpload(w http.ResponseWriter, r *http.Request) {
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

		dstPath := filepath.Join(GetRoot(mode), relPath, fileHeader.Filename)
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
func GenerateHLSPlaylist(playlistPath string, duration float64, segmentDuration float64) error {
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

func HandleStreamStart(w http.ResponseWriter, r *http.Request) {
	targetFile := r.URL.Query().Get("file")
	mode := r.URL.Query().Get("mode")
	root := GetRoot(mode)
	fullPath := filepath.Join(root, targetFile)

	duration, err := GetVideoDuration(fullPath)
	if err != nil {
		http.Error(w, "Failed to get video duration", 500)
		return
	}

	audioCodec := GetAudioCodec(fullPath)
	var audioArgs []string
	if IsAudioCompatible(audioCodec) {
		audioArgs = []string{"-c:a", "copy"}
	} else {
		audioArgs = []string{"-c:a", "aac", "-b:a", "128k"}
	}

	sessionID := fmt.Sprintf("s_%d", time.Now().UnixNano())
	sessionDir := filepath.Join(CachePath, sessionID)
	os.MkdirAll(sessionDir, 0755)

	playlistPath := filepath.Join(sessionDir, "index.m3u8")
	segmentPath := filepath.Join(sessionDir, "seg_%03d.ts")

	// Pre-generate the m3u8 playlist so it's available immediately
	const segmentDuration = 6.0
	if err := GenerateHLSPlaylist(playlistPath, duration, segmentDuration); err != nil {
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

	StreamMutex.Lock()
	ActiveStreams[sessionID] = cmd
	StreamMutex.Unlock()

	// Wait until first segment exists (playlist is already pre-generated)
	firstSeg := filepath.Join(sessionDir, "seg_000.ts")
	firstSegReady := WaitForFile(firstSeg, 50, 200*time.Millisecond)
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

func HandleStreamStop(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		return
	}

	StreamMutex.Lock()
	if cmd, exists := ActiveStreams[sessionID]; exists {
		cmd.Process.Kill()
		cmd.Wait()
		delete(ActiveStreams, sessionID)
		log.Printf("Stopped HLS stream: session=%s", sessionID)
		go os.RemoveAll(filepath.Join(CachePath, sessionID))
	}
	StreamMutex.Unlock()

	w.WriteHeader(200)
}

// Minimal request logger used for HLS so we can see if requests reach the server.
func LogRequests(prefix string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("HLS request [%s]: %s", prefix, r.URL.Path)
		h.ServeHTTP(w, r)
	})
}

// makeHLSHandler serves files from the HLS temp directory with explicit existence checks
// and path cleaning to avoid traversal and to log misses.
func MakeHLSHandler(root string) http.Handler {
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
func WaitForFile(path string, attempts int, sleep time.Duration) bool {
	for i := 0; i < attempts; i++ {
		info, err := os.Stat(path)
		if err == nil && info.Size() > 0 {
			return true
		}
		time.Sleep(sleep)
	}
	return false
}

func GetVideoDuration(filePath string) (float64, error) {
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

func GetAudioCodec(filePath string) string {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "a:0", "-show_entries", "stream=codec_name", "-of", "default=noprint_wrappers=1:nokey=1", filePath)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func IsAudioCompatible(codec string) bool {
	compatible := []string{"aac", "mp3", "ac3", "eac3", "opus"}
	return slices.Contains(compatible, codec)
}
