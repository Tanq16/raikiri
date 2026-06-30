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

// isSubtitleFile reports whether name has a text-based subtitle extension that
// can be converted through ffmpeg -f webvtt. VobSub (.sub/.idx) is image-based
// and intentionally excluded.
func isSubtitleFile(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".srt", ".ass", ".ssa", ".vtt"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func FindExternalSubtitles(videoPath string) []string {
	var subtitles []string
	dir := filepath.Dir(videoPath)

	candidates := []string{dir, filepath.Join(dir, "subs"), filepath.Join(dir, "Subs")}
	for _, scanDir := range candidates {
		log.Printf("DEBUG [media] checking subtitle directory path=%s", scanDir)
		entries, err := os.ReadDir(scanDir)
		if err != nil {
			continue
		}
		for _, f := range entries {
			if !f.IsDir() && isSubtitleFile(f.Name()) {
				log.Printf("DEBUG [media] found subtitle file=%s", f.Name())
				subtitles = append(subtitles, filepath.Join(scanDir, f.Name()))
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
