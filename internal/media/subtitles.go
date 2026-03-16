package media

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// FindExternalSubtitles looks for .srt files in the same directory,
// plus "subs/" and "Subs/" subdirectories.
func FindExternalSubtitles(videoPath string) []string {
	var subtitles []string
	dir := filepath.Dir(videoPath)

	videoDir, err := os.ReadDir(dir)
	if err != nil {
		return subtitles
	}

	for _, f := range videoDir {
		if f.IsDir() || !strings.HasSuffix(strings.ToLower(f.Name()), ".srt") {
			continue
		}
		subtitles = append(subtitles, filepath.Join(dir, f.Name()))
	}

	subsDir := filepath.Join(dir, "subs")
	log.Printf("DEBUG [media] checking subs directory path=%s", subsDir)
	if subsDirEntries, err := os.ReadDir(subsDir); err == nil {
		for _, f := range subsDirEntries {
			log.Printf("DEBUG [media] checking subtitle file=%s", f.Name())
			if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".srt") {
				log.Printf("DEBUG [media] found subtitle file=%s", f.Name())
				subtitles = append(subtitles, filepath.Join(subsDir, f.Name()))
			}
		}
	}

	subsDir = filepath.Join(dir, "Subs")
	log.Printf("DEBUG [media] checking Subs directory path=%s", subsDir)
	if subsDirEntries, err := os.ReadDir(subsDir); err == nil {
		log.Printf("DEBUG [media] files in Subs directory count=%d", len(subsDirEntries))
		for _, f := range subsDirEntries {
			log.Printf("DEBUG [media] checking subtitle file=%s", f.Name())
			if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".srt") {
				log.Printf("DEBUG [media] found subtitle file=%s", f.Name())
				subtitles = append(subtitles, filepath.Join(subsDir, f.Name()))
			}
		}
	}

	return subtitles
}

func GetEmbeddedSubtitleTracks(filePath string) []SubtitleTrack {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=index,codec_name",
		"-of", "csv=p=0",
		filePath)

	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var tracks []SubtitleTrack
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			index, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}
			codec := parts[1]
			textBasedCodecs := []string{"subrip", "ass", "ssa", "webvtt", "mov_text", "srt"}
			if slices.Contains(textBasedCodecs, codec) {
				tracks = append(tracks, SubtitleTrack{Index: index, Codec: codec})
			}
		}
	}

	return tracks
}

func ExtractSubtitleToSRT(videoPath string, streamIndex int, outputPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-f", "webvtt",
		outputPath)

	return cmd.Run()
}

func ConvertSRTtoVTT(srtPath string, vttPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", srtPath,
		"-f", "webvtt",
		vttPath)

	return cmd.Run()
}
