package thumbnails

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tanq16/raikiri/internal/media"
	u "github.com/tanq16/raikiri/utils"
)

func CreateVideoThumbnail(filePath string) error {
	dir := filepath.Dir(filePath)
	filename := filepath.Base(filePath)
	thumbFilename := fmt.Sprintf(".%s.thumbnail.jpg", filename)
	thumbPath := filepath.Join(dir, thumbFilename)

	if _, err := os.Stat(thumbPath); err == nil {
		if !askToOverwrite(thumbPath) {
			return nil
		}
	}

	duration, err := media.GetVideoDuration(filePath)
	if err != nil {
		return fmt.Errorf("failed to get video duration: %w", err)
	}

	seekTime := duration / 2.0
	if seekTime >= duration {
		seekTime = max(0.0, duration-0.5)
	}
	seekTimeStr := media.FormatDuration(seekTime)

	cmd := exec.Command("ffmpeg", "-ss", seekTimeStr, "-i", filePath, "-vframes", "1", "-vf", "scale=400:-1", "-q:v", "3", "-y", thumbPath)
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		stderrContent := strings.TrimSpace(stderrBuf.String())
		if stderrContent != "" {
			return fmt.Errorf("%s: %w", stderrContent, err)
		}
		return err
	}
	return nil
}

func ProcessVideos(rootDir string) {
	var filesToProcess []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasPrefix(info.Name(), ".") {
			ext := strings.ToLower(filepath.Ext(info.Name()))
			if slices.Contains(media.VideoExtensions, ext) {
				filesToProcess = append(filesToProcess, path)
			}
		}
		return nil
	})
	if err != nil {
		u.PrintError("error walking directory", err)
		return
	}

	u.PrintInfo(fmt.Sprintf("found %d video files in '%s'", len(filesToProcess), rootDir))
	for i, filePath := range filesToProcess {
		u.PrintInfo(fmt.Sprintf("[%d/%d] processing: %s", i+1, len(filesToProcess), filepath.Base(filePath)))
		err := CreateVideoThumbnail(filePath)
		if err != nil {
			u.PrintError("thumbnail creation failed", err)
		}
	}
}

func ProcessVideo(currentDir string) {
	var filesToProcess []string

	entries, err := os.ReadDir(currentDir)
	if err != nil {
		u.PrintError("error reading directory", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if slices.Contains(media.VideoExtensions, ext) {
			filePath := filepath.Join(currentDir, entry.Name())
			filesToProcess = append(filesToProcess, filePath)
		}
	}

	u.PrintInfo(fmt.Sprintf("found %d video files in '%s'", len(filesToProcess), currentDir))
	for i, filePath := range filesToProcess {
		u.PrintInfo(fmt.Sprintf("[%d/%d] processing: %s", i+1, len(filesToProcess), filepath.Base(filePath)))
		err := CreateVideoThumbnail(filePath)
		if err != nil {
			u.PrintError("thumbnail creation failed", err)
		}
	}
}
