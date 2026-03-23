package video

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type EncodeOptions struct {
	Quality string
	Faster  bool
}

var qualityCRF = map[string]string{
	"very-high": "22",
	"high":      "24",
	"medium":    "26",
	"low":       "28",
}

var bitmapSubCodecs = map[string]bool{
	"hdmv_pgs_subtitle": true,
	"vobsub":            true,
	"dvd_subtitle":      true,
}

var commentaryRegex = regexp.MustCompile(`(?i)commentary|director|cast`)

type indexedStream struct {
	relIdx int
	stream Stream
}

func RunEncode(inputFile string, opts EncodeOptions) error {
	data, err := getVideoInfo(inputFile)
	if err != nil {
		return err
	}

	args, outputFile, err := buildFFmpegArgs(inputFile, data, opts)
	if err != nil {
		return err
	}

	fmt.Printf("Command: ffmpeg %s\n\n", strings.Join(args, " "))

	return runEncode(outputFile, data, args)
}

func buildFFmpegArgs(inputFile string, data *FFProbeOutput, opts EncodeOptions) ([]string, string, error) {
	args := []string{"-i", inputFile}

	videoStreams := filterStreams(data.Streams, "video")
	if len(videoStreams) == 0 {
		return nil, "", fmt.Errorf("no video streams found in input")
	}

	args = append(args, "-map", "0:v:0")

	crf, ok := qualityCRF[opts.Quality]
	if !ok {
		crf = qualityCRF["medium"]
	}

	preset := "slow"
	if opts.Faster {
		preset = "medium"
	}

	videoFlags := []string{"-c:v", "libx265", "-crf", crf, "-preset", preset, "-fps_mode", "cfr"}

	// Retain 10-bit pixel format if source is 10-bit
	if videoStreams[0].stream.PixFmt == "yuv420p10le" {
		videoFlags = append(videoFlags, "-pix_fmt", "yuv420p10le")
		fmt.Println("→ 10-bit source detected, retaining pixel format")
	}

	// Auto-halve frame rates above 30 fps
	fpsTarget := computeFPSTarget(videoStreams[0].stream.AvgFrameRate)
	if fpsTarget != "" {
		videoFlags = append(videoFlags, "-r", fpsTarget)
		fmt.Printf("→ FPS auto-halved to %s\n", fpsTarget)
	}

	// hvc1 tag for Safari/iOS compatibility
	videoFlags = append(videoFlags, "-tag:v", "hvc1")

	fmt.Printf("→ Video: libx265 CRF %s (%s quality, preset %s, CFR)\n", crf, opts.Quality, preset)

	// Audio: always re-encode to AAC stereo 48kHz
	var audioFlags []string
	audioStreams := filterStreams(data.Streams, "audio")

	if len(audioStreams) > 0 {
		selectedIdx := selectAudioStream(audioStreams)
		args = append(args, "-map", fmt.Sprintf("0:a:%d", selectedIdx))

		audioFlags = append(audioFlags, "-c:a", "aac", "-ac", "2", "-ar", "48000", "-b:a", "192k")

		selected := audioStreams[selectedIdx]
		lang := selected.stream.Tags.Language
		if lang == "" {
			lang = "und"
		}
		if selected.stream.Tags.Title != "" {
			fmt.Printf("→ Audio: stream #%d (%s — %s) → AAC stereo 48kHz 192k\n", selected.stream.Index, lang, selected.stream.Tags.Title)
		} else {
			fmt.Printf("→ Audio: stream #%d (%s) → AAC stereo 48kHz 192k\n", selected.stream.Index, lang)
		}
	} else {
		fmt.Println("→ Audio: none")
	}

	// Subtitles: keep all, pick container based on codec type
	var subtitleFlags []string
	subStreams := filterStreams(data.Streams, "subtitle")
	outputExt := ".mp4"

	if len(subStreams) > 0 {
		hasBitmap := false
		for _, ss := range subStreams {
			if bitmapSubCodecs[ss.stream.CodecName] {
				hasBitmap = true
				break
			}
		}

		for i := range subStreams {
			args = append(args, "-map", fmt.Sprintf("0:s:%d", i))
		}

		if hasBitmap {
			outputExt = ".mkv"
			subtitleFlags = append(subtitleFlags, "-c:s", "copy")
			fmt.Printf("→ Subtitles: %d stream(s) (bitmap detected → MKV, copy)\n", len(subStreams))
		} else {
			subtitleFlags = append(subtitleFlags, "-c:s", "mov_text")
			fmt.Printf("→ Subtitles: %d stream(s) (text → MP4, mov_text)\n", len(subStreams))
		}
	} else {
		fmt.Println("→ Subtitles: none")
	}

	dir := filepath.Dir(inputFile)
	base := strings.TrimSuffix(filepath.Base(inputFile), filepath.Ext(inputFile))
	outputFile := filepath.Join(dir, base+".h265"+outputExt)

	args = append(args, videoFlags...)
	args = append(args, audioFlags...)
	args = append(args, subtitleFlags...)
	args = append(args, "-avoid_negative_ts", "make_zero")
	args = append(args, "-movflags", "+faststart")
	args = append(args, outputFile)

	fmt.Printf("→ Output: %s\n", outputFile)

	return args, outputFile, nil
}

// Standard high frame rates and their halved targets.
// Only these are auto-halved to avoid introducing non-standard rates.
// Map key is "num/den" rational form, value is the halved target.
var standardFPSHalving = map[string]string{
	"60/1":       "30/1",       // 60 → 30
	"60000/1001": "30000/1001", // 59.94 → 29.97
	"50/1":       "25/1",       // 50 → 25
	"48/1":       "24/1",       // 48 → 24
	"48000/1001": "24000/1001", // 47.95 → 23.976
}

// computeFPSTarget returns the halved frame rate for known standard high frame rates.
// Returns empty string if the source is not a recognized standard rate above 30 fps.
func computeFPSTarget(avgFrameRate string) string {
	parts := strings.Split(avgFrameRate, "/")
	if len(parts) != 2 {
		return ""
	}

	num, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || num == 0 {
		return ""
	}
	den, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || den == 0 {
		return ""
	}

	// Normalize to simplest form for lookup
	g := gcd(num, den)
	normalized := fmt.Sprintf("%d/%d", num/g, den/g)

	if target, ok := standardFPSHalving[normalized]; ok {
		return target
	}

	return ""
}

func gcd(a, b int64) int64 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func filterStreams(streams []Stream, codecType string) []indexedStream {
	var result []indexedStream
	for _, s := range streams {
		if s.CodecType == codecType {
			result = append(result, indexedStream{relIdx: len(result), stream: s})
		}
	}
	return result
}

func selectAudioStream(audioStreams []indexedStream) int {
	if len(audioStreams) == 1 {
		return 0
	}

	for i, as := range audioStreams {
		if isRejectedAudio(as.stream) {
			continue
		}
		lang := as.stream.Tags.Language
		if lang == "eng" || lang == "" {
			return i
		}
	}

	for i, as := range audioStreams {
		if isRejectedAudio(as.stream) {
			continue
		}
		return i
	}

	return 0
}

func isRejectedAudio(s Stream) bool {
	if commentaryRegex.MatchString(s.Tags.Title) {
		return true
	}
	if s.Disposition.Comment == 1 || s.Disposition.VisualImpaired == 1 {
		return true
	}
	return false
}

func runEncode(outputFile string, data *FFProbeOutput, ffmpegArgs []string) error {
	totalDurationSecs := 0.0
	if data.Format.Duration != "" {
		totalDurationSecs, _ = strconv.ParseFloat(data.Format.Duration, 64)
	}

	ffmpegArgs = append(ffmpegArgs, "-progress", "pipe:1", "-nostats", "-loglevel", "error", "-y")

	cmd := exec.Command("ffmpeg", ffmpegArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	errorChan := make(chan bool, 1)
	go func() {
		hasErrors := false
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" && isErrorLine(line) {
				hasErrors = true
				fmt.Fprintf(os.Stderr, "\r\033[K%s\n", line)
			}
		}
		errorChan <- hasErrors
	}()

	fmt.Printf("Encoding: %s | Duration: %s\n", outputFile, formatDuration(totalDurationSecs))
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "=")

		if len(parts) == 2 && parts[0] == "out_time_us" {
			currentUs, _ := strconv.ParseFloat(parts[1], 64)
			currentSecs := currentUs / 1000000.0

			if totalDurationSecs > 0 {
				percent := (currentSecs / totalDurationSecs) * 100
				if percent > 100 {
					percent = 100
				}
				drawProgressBar(percent, currentSecs, totalDurationSecs)
			} else {
				fmt.Printf("\rEncoding... %.1fs", currentSecs)
			}
		}
	}

	cmdErr := cmd.Wait()
	errorsDetected := <-errorChan

	if cmdErr != nil || errorsDetected {
		fmt.Println()
		if cmdErr != nil {
			return fmt.Errorf("ffmpeg encoding failed: %w", cmdErr)
		}
		return fmt.Errorf("encoding completed with errors (see messages above)")
	}

	fmt.Printf("\r\033[KEncoding completed in %s\n\n", time.Since(startTime))
	return nil
}

func drawProgressBar(percent float64, current, total float64) {
	width := 40
	completed := min(int((percent/100)*float64(width)), width)

	filled := strings.Repeat("━", completed)
	empty := strings.Repeat(" ", width-completed)

	fmt.Printf("\r[%s%s] %.1f%% (%.1fs / %.1fs)", filled, empty, percent, current, total)
}

func isErrorLine(line string) bool {
	line = strings.ToLower(line)
	if strings.Contains(line, "[info]") || strings.Contains(line, "[warning]") {
		return false
	}
	if strings.Contains(line, "error") || strings.Contains(line, "failed") || strings.Contains(line, "cannot") {
		return true
	}
	return false
}
