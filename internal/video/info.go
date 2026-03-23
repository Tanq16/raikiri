package video

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type FFProbeOutput struct {
	Streams []Stream `json:"streams"`
	Format  Format   `json:"format"`
}

type Stream struct {
	Index         int         `json:"index"`
	CodecType     string      `json:"codec_type"`
	CodecName     string      `json:"codec_name"`
	Width         int         `json:"width,omitempty"`
	Height        int         `json:"height,omitempty"`
	BitRate       string      `json:"bit_rate,omitempty"`
	AvgFrameRate  string      `json:"avg_frame_rate,omitempty"`
	RFrameRate    string      `json:"r_frame_rate,omitempty"`
	PixFmt        string      `json:"pix_fmt,omitempty"`
	Channels      int         `json:"channels,omitempty"`
	ChannelLayout string      `json:"channel_layout,omitempty"`
	SampleRate    string      `json:"sample_rate,omitempty"`
	Tags          Tags        `json:"tags,omitempty"`
	Disposition   Disposition `json:"disposition"`
}

type Disposition struct {
	Comment        int `json:"comment"`
	VisualImpaired int `json:"visual_impaired"`
}

type Format struct {
	Filename   string `json:"filename"`
	Duration   string `json:"duration"`
	Size       string `json:"size"`
	BitRate    string `json:"bit_rate"`
	FormatName string `json:"format_name"`
}

type Tags struct {
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
	BPS      string `json:"BPS,omitempty"`
}

func RunVideoInfo(inputFile string) error {
	data, err := getVideoInfo(inputFile)
	if err != nil {
		return err
	}

	printOverview(data.Format)
	printStreams(data.Streams)
	return nil
}

func getVideoInfo(inputFile string) (*FFProbeOutput, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputFile,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ffprobe: %w", err)
	}

	var data FFProbeOutput
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	return &data, nil
}

func printOverview(f Format) {
	sizeBytes, _ := strconv.ParseFloat(f.Size, 64)
	durationSec, _ := strconv.ParseFloat(f.Duration, 64)
	bitrate, _ := strconv.ParseFloat(f.BitRate, 64)
	fmt.Printf(" Container: %s  |  Size: %s  |  Duration: %s  |  Bitrate: %s\n",
		f.FormatName, formatSize(sizeBytes), formatDuration(durationSec), formatBitrate(bitrate))
	fmt.Println("")
}

func printStreams(streams []Stream) {
	fmt.Println("STREAMS:")
	for _, s := range streams {
		switch s.CodecType {
		case "video":
			fps := parseFrameRate(s.AvgFrameRate)
			bitrate := ""
			if s.Tags.BPS != "" {
				br, _ := strconv.ParseFloat(s.Tags.BPS, 64)
				bitrate = fmt.Sprintf(" | Bitrate: %s", formatBitrate(br))
			}
			fmt.Printf(" [VIDEO #%d] %s\n", s.Index, strings.ToUpper(s.CodecName))
			fmt.Printf("   %dx%d | %s fps | %s%s\n", s.Width, s.Height, fps, s.PixFmt, bitrate)

		case "audio":
			lang := s.Tags.Language
			if lang == "" {
				lang = "und"
			}
			bitrate := ""
			if s.Tags.BPS != "" {
				br, _ := strconv.ParseFloat(s.Tags.BPS, 64)
				bitrate = fmt.Sprintf(" | %s", formatBitrate(br))
			}
			fmt.Printf(" [AUDIO #%d] %s | %s\n", s.Index, strings.ToUpper(s.CodecName), strings.ToUpper(lang))
			fmt.Printf("   %d ch (%s) | %s Hz%s\n", s.Channels, s.ChannelLayout, s.SampleRate, bitrate)
			if s.Tags.Title != "" {
				fmt.Printf("   %s\n", s.Tags.Title)
			}

		case "subtitle":
			lang := s.Tags.Language
			if lang == "" {
				lang = "und"
			}
			fmt.Printf(" [SUB #%d] %s | %s", s.Index, strings.ToUpper(s.CodecName), strings.ToUpper(lang))
			if s.Tags.Title != "" {
				fmt.Printf(" | %s", s.Tags.Title)
			}
			fmt.Println()
		}
		fmt.Println()
	}
}

func formatSize(bytes float64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%.0f B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", bytes/float64(div), "KMGTPE"[exp])
}

func formatBitrate(bps float64) string {
	return fmt.Sprintf("%.2f Mbps", bps/1000000)
}

func formatDuration(seconds float64) string {
	d := time.Duration(seconds) * time.Second
	return d.String()
}

func parseFrameRate(fr string) string {
	parts := strings.Split(fr, "/")
	if len(parts) == 2 {
		num, _ := strconv.ParseFloat(parts[0], 64)
		den, _ := strconv.ParseFloat(parts[1], 64)
		if den > 0 {
			return fmt.Sprintf("%.2f", num/den)
		}
	}
	return fr
}
