package media

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

var VideoExtensions = []string{".mp4", ".mkv", ".webm", ".mov", ".avi"}

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

func FormatDuration(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := int(seconds) % 3600 / 60
	secs := int(seconds) % 60
	frac := seconds - float64(int(seconds))
	millis := int(frac * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, millis)
}
