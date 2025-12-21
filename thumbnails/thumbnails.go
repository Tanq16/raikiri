package thumbnails

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tanq16/raikiri/handlers"
)

func FormatDuration(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := int(seconds) % 3600 / 60
	secs := int(seconds) % 60
	frac := seconds - float64(int(seconds))
	millis := int(frac * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, millis)
}

func CreateVideoThumbnail(filePath string) error {
	dir := filepath.Dir(filePath)
	filename := filepath.Base(filePath)
	thumbFilename := fmt.Sprintf(".%s.thumbnail.jpg", filename)
	thumbPath := filepath.Join(dir, thumbFilename)

	// Skip if thumbnail already exists
	if _, err := os.Stat(thumbPath); err == nil {
		return nil
	}

	// Get video duration
	duration, err := handlers.GetVideoDuration(filePath)
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
	seekTimeStr := FormatDuration(seekTime)

	// Create thumbnail at 50% of video duration with -ss before -i for fast input seeking
	cmd := exec.Command("ffmpeg", "-ss", seekTimeStr, "-i", filePath, "-vframes", "1", "-vf", "scale=400:-1", "-q:v", "3", "-y", thumbPath)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create thumbnail for %s: %w", filename, err)
	}
	return nil
}

func IsVideoFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	videoExts := []string{".mp4", ".mkv", ".webm", ".mov", ".avi"}
	return slices.Contains(videoExts, ext)
}

func ProcessDirectoryForThumbnails(rootDir string) {
	var filesToProcess []string
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasPrefix(info.Name(), ".") {
			if IsVideoFile(info.Name()) {
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
		err := CreateVideoThumbnail(filePath)
		if err != nil {
			fmt.Printf("\nERROR: %s - %v\n", filePath, err)
		} else {
			fmt.Printf("\r%d / %d files done", i+1, totalFiles)
		}
	}
	fmt.Println()
}
