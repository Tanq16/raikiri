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
)

//go:embed public
var staticFiles embed.FS

var (
	mediaPath string
	musicPath string
)

type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"` // Relative path from root of mode
	Type  string `json:"type"` // folder, audio, video, image, other
	Size  string `json:"size"`
	Thumb string `json:"thumb,omitempty"`
}

func main() {
	prepare := flag.Bool("prepare", false, "Generate thumbnails for all video files and exit")
	flag.StringVar(&mediaPath, "media", ".", "Path to media directory")
	flag.StringVar(&musicPath, "music", "./music", "Path to music directory")
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

	http.Handle("/", http.FileServer(http.FS(mustSub(staticFiles, "public"))))
	http.HandleFunc("/api/list", handleList)
	http.HandleFunc("/api/upload", handleUpload)
	http.HandleFunc("/content/", handleContent)

	fmt.Printf("Raikiri running on :8080\nMedia: %s\nMusic: %s\n", mediaPath, musicPath)
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
			if err != nil || d.IsDir() {
				return nil
			}
			// Skip hidden files/thumbnails
			if strings.HasPrefix(d.Name(), ".") {
				return nil
			}

			fType := getFileType(d.Name(), false)
			if fType == "audio" || fType == "video" || fType == "image" {
				rel, _ := filepath.Rel(root, path)
				rel = filepath.ToSlash(rel) // Force forward slash

				info, err := d.Info()
				size := ""
				if err == nil {
					size = fmt.Sprintf("%.1f MB", float64(info.Size())/1024/1024)
				}

				entries = append(entries, FileEntry{
					Name: d.Name(),
					Path: rel,
					Type: fType,
					Size: size,
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

			fType := getFileType(f.Name(), f.IsDir())

			// Generate relative path from ROOT, not from current folder
			fullRelPath := filepath.Join(relPath, f.Name())
			fullRelPath = filepath.ToSlash(fullRelPath)

			// Determine thumbnail path logic
			thumbPath := ""
			if f.IsDir() {
				thumbPath = filepath.Join(relPath, f.Name(), ".thumbnail.jpg")
			} else if fType == "video" || fType == "image" || fType == "audio" {
				thumbPath = filepath.Join(relPath, "."+f.Name()+".thumbnail.jpg")
			}
			thumbPath = filepath.ToSlash(thumbPath)

			entries = append(entries, FileEntry{
				Name:  f.Name(),
				Path:  fullRelPath,
				Type:  fType,
				Size:  size,
				Thumb: thumbPath,
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

// Thumbnail Generation

func getVideoDuration(filePath string) (float64, error) {
	// Use ffprobe to get video duration
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
