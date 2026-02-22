package media

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// VideoExtensions is the canonical list of video file extensions.
var VideoExtensions = []string{".mp4", ".mkv", ".webm", ".mov", ".avi"}

// GetFileType classifies a file/directory into a UI type string.
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

// GetThumbnailPath returns the expected thumbnail path for a file or folder.
func GetThumbnailPath(relPath, fileName, fileType, mode string) string {
	if fileType == "folder" {
		return filepath.ToSlash(filepath.Join(relPath, ".thumbnail.jpg"))
	}
	if mode == "music" && fileType == "audio" {
		return filepath.ToSlash(filepath.Join(relPath, ".thumbnail.jpg"))
	}
	return filepath.ToSlash(filepath.Join(relPath, "."+fileName+".thumbnail.jpg"))
}

// FormatFileSize formats bytes as a human-readable "X.X MB" string.
func FormatFileSize(bytes int64) string {
	return fmt.Sprintf("%.1f MB", float64(bytes)/1024/1024)
}

// FormatModTime formats a time.Time for display.
func FormatModTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

// FormatDuration converts seconds to HH:MM:SS.mmm format.
func FormatDuration(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := int(seconds) % 3600 / 60
	secs := int(seconds) % 60
	frac := seconds - float64(int(seconds))
	millis := int(frac * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, millis)
}
