package media

import (
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// GetAudioDuration returns the duration of an audio file in seconds.
// ffprobe's format=duration works for audio and video, so this aliases GetVideoDuration.
func GetAudioDuration(filePath string) (float64, error) {
	return GetVideoDuration(filePath)
}

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
		"-show_entries", "stream=index,codec_name,profile,channels:stream_tags=language",
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
		if len(parts) >= 4 {
			index, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}
			codec := parts[1]
			profile := parts[2]
			channels, err := strconv.Atoi(parts[3])
			if err != nil {
				channels = 2
			}
			language := "und"
			if len(parts) >= 5 {
				language = parts[4]
			}
			tracks = append(tracks, AudioTrack{
				Index:    index,
				Codec:    codec,
				Profile:  profile,
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

func IsAudioCompatible(codec string) bool {
	compatible := []string{"aac", "mp3", "opus"}
	return slices.Contains(compatible, codec)
}

func IsVideoCompatibleForHLS(codec string) bool {
	compatible := []string{"h264", "avc", "hevc", "h265"}
	return slices.Contains(compatible, codec)
}

func GetContainerFormat(filePath string) string {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=format_name", "-of", "default=noprint_wrappers=1:nokey=1", filePath)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func GetAudioSampleRate(filePath string, streamIndex int) int {
	cmd := exec.Command("ffprobe", "-v", "error",
		"-select_streams", fmt.Sprintf("%d", streamIndex),
		"-show_entries", "stream=sample_rate",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath)
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	rate, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0
	}
	return rate
}

// Requires: MP4/MOV container, HLS-compatible video, compatible audio, stereo, 48kHz.
func IsDirectServable(filePath string) bool {
	format := GetContainerFormat(filePath)
	if !strings.Contains(format, "mp4") && !strings.Contains(format, "mov") {
		return false
	}

	videoCodec := GetVideoCodec(filePath)
	if !IsVideoCompatibleForHLS(videoCodec) {
		return false
	}

	tracks := GetAudioTracks(filePath)
	selected := SelectBestAudioTrack(tracks)
	if selected == nil {
		return true // no audio is fine
	}

	if !IsAudioCompatible(selected.Codec) {
		return false
	}
	if selected.Channels > 2 {
		return false
	}

	sampleRate := GetAudioSampleRate(filePath, selected.Index)
	if sampleRate != 48000 {
		return false
	}

	return true
}
