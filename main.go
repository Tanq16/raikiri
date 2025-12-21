package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/tanq16/raikiri/handlers"
	"github.com/tanq16/raikiri/thumbnails"
)

//go:embed public
var staticFiles embed.FS

var (
	mediaPath string
	musicPath string
	cachePath string
)

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
		thumbnails.ProcessDirectoryForThumbnails(cwd)
		log.Println("Thumbnail generation complete.")
		return
	}

	// Initialize handler package variables
	handlers.MediaPath = mediaPath
	handlers.MusicPath = musicPath
	handlers.CachePath = cachePath

	// Clean up and create cache directory
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		log.Fatalf("Failed to create cache directory: %v", err)
	}

	http.HandleFunc("/api/list", handlers.HandleList)
	http.HandleFunc("/api/stream", handlers.HandleStreamStart)
	http.HandleFunc("/api/stop-stream", handlers.HandleStreamStop)
	http.HandleFunc("/api/upload", handlers.HandleUpload)
	http.HandleFunc("/content/", handlers.HandleContent)

	hlsHandler := handlers.MakeHLSHandler(cachePath)
	http.Handle("/hls/", http.StripPrefix("/hls/", handlers.LogRequests("hls", hlsHandler)))
	http.Handle("/api/hls/", http.StripPrefix("/api/hls/", handlers.LogRequests("api/hls", hlsHandler)))

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
