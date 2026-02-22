package media

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func plog() *zerolog.Logger {
	l := log.With().Str("package", "media").Logger()
	return &l
}

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
	plog().Debug().Str("path", subsDir).Msg("checking subs directory")
	if subsDirEntries, err := os.ReadDir(subsDir); err == nil {
		for _, f := range subsDirEntries {
			plog().Debug().Str("file", f.Name()).Msg("checking subtitle")
			if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".srt") {
				plog().Debug().Str("file", f.Name()).Msg("found subtitle")
				subtitles = append(subtitles, filepath.Join(subsDir, f.Name()))
			}
		}
	}

	subsDir = filepath.Join(dir, "Subs")
	plog().Debug().Str("path", subsDir).Msg("checking Subs directory")
	if subsDirEntries, err := os.ReadDir(subsDir); err == nil {
		plog().Debug().Int("count", len(subsDirEntries)).Msg("files in Subs directory")
		for _, f := range subsDirEntries {
			plog().Debug().Str("file", f.Name()).Msg("checking subtitle")
			if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".srt") {
				plog().Debug().Str("file", f.Name()).Msg("found subtitle")
				subtitles = append(subtitles, filepath.Join(subsDir, f.Name()))
			}
		}
	}

	return subtitles
}

// GetEmbeddedSubtitleTracks returns text-based subtitle tracks embedded in the file.
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

// ExtractSubtitleToSRT extracts a subtitle stream to WebVTT format.
func ExtractSubtitleToSRT(videoPath string, streamIndex int, outputPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-f", "webvtt",
		outputPath)

	return cmd.Run()
}

// ConvertSRTtoVTT converts an SRT file to WebVTT format.
func ConvertSRTtoVTT(srtPath string, vttPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", srtPath,
		"-f", "webvtt",
		vttPath)

	return cmd.Run()
}
