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
	prepareMode := flag.String("prepare", "", "Mode: 'videos' (generate ffmpeg thumbs recursively), 'video' (generate ffmpeg thumbs in current folder), 'shows' (auto-match all subdirs), 'show' (manual interactive match current dir)")

	flag.StringVar(&mediaPath, "media", ".", "Path to media directory")
	flag.StringVar(&musicPath, "music", "./music", "Path to music directory")
	flag.StringVar(&cachePath, "cache", "/tmp", "Path to cache directory for HLS segments")
	flag.Parse()

	if *prepareMode != "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Error getting current working directory: %v", err)
		}

		if *prepareMode == "videos" || *prepareMode == "video" {
			if _, err := exec.LookPath("ffmpeg"); err != nil {
				log.Fatalf("Error: `ffmpeg` not found in PATH.")
			}
			if _, err := exec.LookPath("ffprobe"); err != nil {
				log.Fatalf("Error: `ffprobe` not found in PATH.")
			}
			log.Println("Starting video thumbnail generation...")
			switch *prepareMode {
			case "videos":
				thumbnails.ProcessVideos(cwd)
			case "video":
				thumbnails.ProcessVideo(cwd)
			}
			log.Println("Complete.")
			return
		}

		// Check TMDB Key for Show modes
		if *prepareMode == "shows" || *prepareMode == "show" {
			apiKey := os.Getenv("TMDB_API_KEY")
			if apiKey == "" {
				log.Fatal("Error: TMDB_API_KEY environment variable is required for show metadata.")
			}
			thumbnails.TmdbAPIKey = apiKey

			switch *prepareMode {
			case "shows":
				log.Println("Starting automatic show processing...")
				thumbnails.ProcessShowsAuto(cwd)
			case "show":
				log.Println("Starting manual show processing...")
				thumbnails.ProcessShowManual(cwd)
			}
			log.Println("Complete.")
			return
		}

		log.Fatalf("Invalid prepare mode: '%s'. Use 'videos', 'video', 'shows', or 'show'.", *prepareMode)
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
