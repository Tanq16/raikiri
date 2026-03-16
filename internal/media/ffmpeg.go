package media

import (
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

func GetVideoDuration(filePath string) (float64, error) {
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

func GetAudioTracks(filePath string) []AudioTrack {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "a",
		"-show_entries", "stream=index,codec_name,channels:stream_tags=language",
		"-of", "csv=p=0",
		filePath)

	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var tracks []AudioTrack
	lines := strings.SplitSeq(strings.TrimSpace(string(output)), "\n")
	for line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) >= 3 {
			index, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}
			codec := parts[1]
			channels, err := strconv.Atoi(parts[2])
			if err != nil {
				channels = 2
			}
			language := "und"
			if len(parts) >= 4 {
				language = parts[3]
			}
			tracks = append(tracks, AudioTrack{
				Index:    index,
				Codec:    codec,
				Language: language,
				Channels: channels,
			})
		}
	}

	return tracks
}

func SelectBestAudioTrack(tracks []AudioTrack) *AudioTrack {
	if len(tracks) == 0 {
		return nil
	}
	for _, track := range tracks {
		if track.Language == "eng" || track.Language == "en" {
			return &track
		}
	}
	return &tracks[0]
}

func GetVideoCodec(filePath string) string {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=codec_name", "-of", "default=noprint_wrappers=1:nokey=1", filePath)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// IsVideoCompatibleForHLS returns true if the codec can be muxed into HLS without transcoding.
func IsVideoCompatibleForHLS(codec string) bool {
	compatible := []string{"h264", "avc", "hevc", "h265"}
	return slices.Contains(compatible, codec)
}
