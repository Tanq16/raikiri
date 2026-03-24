package video

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	u "github.com/tanq16/raikiri/utils"
)

type EncodeOptions struct {
	Quality string
	Slower  bool
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

type encodeResult struct {
	args       []string
	outputFile string
	details    []string // indented detail lines for video info
}

func RunEncode(ctx context.Context, inputFile string, opts EncodeOptions) error {
	data, err := getVideoInfo(inputFile)
	if err != nil {
		return err
	}

	result, err := buildFFmpegArgs(inputFile, data, opts)
	if err != nil {
		return err
	}

	return runEncode(ctx, result, data)
}

func buildFFmpegArgs(inputFile string, data *FFProbeOutput, opts EncodeOptions) (*encodeResult, error) {
	args := []string{"-i", inputFile}
	var details []string

	videoStreams := filterStreams(data.Streams, "video")
	if len(videoStreams) == 0 {
		return nil, fmt.Errorf("no video streams found in input")
	}

	args = append(args, "-map", "0:v:0")

	crf, ok := qualityCRF[opts.Quality]
	if !ok {
		crf = qualityCRF["medium"]
	}

	preset := "medium"
	if opts.Slower {
		preset = "slow"
	}

	videoFlags := []string{"-c:v", "libx265", "-crf", crf, "-preset", preset, "-fps_mode", "cfr"}

	if videoStreams[0].stream.PixFmt == "yuv420p10le" {
		videoFlags = append(videoFlags, "-pix_fmt", "yuv420p10le")
		details = append(details, "10-bit source detected, retaining pixel format")
	}

	fpsTarget := computeFPSTarget(videoStreams[0].stream.AvgFrameRate)
	if fpsTarget == "" {
		fpsTarget = computeFPSTarget(videoStreams[0].stream.RFrameRate)
	}
	if fpsTarget != "" {
		videoFlags = append(videoFlags, "-r", fpsTarget)
		details = append(details, fmt.Sprintf("FPS auto-halved to %s", fpsTarget))
	}

	videoFlags = append(videoFlags, "-tag:v", "hvc1")

	details = append(details, fmt.Sprintf("Video: libx265 CRF %s (%s quality, preset %s, CFR)", crf, opts.Quality, preset))

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
			details = append(details, fmt.Sprintf("Audio: stream #%d (%s — %s) → AAC stereo 48kHz 192k", selected.stream.Index, lang, selected.stream.Tags.Title))
		} else {
			details = append(details, fmt.Sprintf("Audio: stream #%d (%s) → AAC stereo 48kHz 192k", selected.stream.Index, lang))
		}
	} else {
		details = append(details, "Audio: none")
	}

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
			details = append(details, fmt.Sprintf("Subtitles: %d stream(s) (bitmap detected → MKV, copy)", len(subStreams)))
		} else {
			subtitleFlags = append(subtitleFlags, "-c:s", "mov_text")
			details = append(details, fmt.Sprintf("Subtitles: %d stream(s) (text → MP4, mov_text)", len(subStreams)))
		}
	} else {
		details = append(details, "Subtitles: none")
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

	details = append(details, fmt.Sprintf("Output: %s", outputFile))

	return &encodeResult{args: args, outputFile: outputFile, details: details}, nil
}

// formatCommand splits the ffmpeg arg list into logical multi-line groups.
// Returns the formatted lines (each ending with \) and the line count.
func formatCommand(args []string) ([]string, int) {
	var lines []string
	var current []string

	// Group args: start a new line at each -map, -c:v, -c:a, -c:s, -avoid_negative_ts, -f,
	// or when current line gets the output file (last arg with no dash prefix)
	breakBefore := map[string]bool{
		"-map": true, "-c:v": true, "-c:a": true, "-c:s": true,
		"-avoid_negative_ts": true, "-hls_time": true, "-f": true,
	}

	// First line always starts with "ffmpeg"
	current = append(current, "ffmpeg")

	for i, arg := range args {
		isLast := i == len(args)-1

		if breakBefore[arg] && len(current) > 1 {
			lines = append(lines, "  "+strings.Join(current, " ")+" \\")
			current = nil
		}

		current = append(current, arg)

		if isLast {
			lines = append(lines, "  "+strings.Join(current, " "))
		}
	}

	if len(lines) == 0 && len(current) > 0 {
		lines = append(lines, "  "+strings.Join(current, " "))
	}

	return lines, len(lines)
}

// Standard high frame rates and their halved targets.
// Map key is "num/den" rational form for exact matching.
var standardFPSHalving = map[string]string{
	"60/1":       "30/1",       // 60 → 30
	"60000/1001": "30000/1001", // 59.94 → 29.97
	"50/1":       "25/1",       // 50 → 25
	"48/1":       "24/1",       // 48 → 24
	"48000/1001": "24000/1001", // 47.95 → 23.976
}

// Float-based fallback for containers that report imprecise frame rates
// (e.g. 1293975/21566 ≈ 60.0007 instead of 60/1). Epsilon of 0.002 is
// tight enough to never confuse adjacent standards (nearest gap is 0.06
// between 59.94 and 60.0) while catching container metadata noise.
var fpsFloatHalving = []struct {
	fps    float64
	target string
}{
	{60.0, "30/1"},
	{59.94005994, "30000/1001"}, // 60000/1001
	{50.0, "25/1"},
	{48.0, "24/1"},
	{47.95204796, "24000/1001"}, // 48000/1001
}

const fpsMatchEpsilon = 0.002

// computeFPSTarget returns the halved frame rate for known standard high frame rates.
// Tries exact rational match first, then falls back to float comparison within
// 0.002 fps tolerance for imprecise container metadata.
// Returns empty string if the source is not a recognized rate above 30 fps.
func computeFPSTarget(frameRate string) string {
	parts := strings.Split(frameRate, "/")
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

	g := gcd(num, den)
	normalized := fmt.Sprintf("%d/%d", num/g, den/g)
	if target, ok := standardFPSHalving[normalized]; ok {
		return target
	}

	fps := float64(num) / float64(den)
	for _, entry := range fpsFloatHalving {
		diff := fps - entry.fps
		if diff < 0 {
			diff = -diff
		}
		if diff < fpsMatchEpsilon {
			return entry.target
		}
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

func runEncode(ctx context.Context, result *encodeResult, data *FFProbeOutput) error {
	totalDurationSecs := 0.0
	if data.Format.Duration != "" {
		totalDurationSecs, _ = strconv.ParseFloat(data.Format.Duration, 64)
	}

	// Print video information phase
	lineCount := 0
	u.PrintInfo("Video Information")
	lineCount++
	for _, detail := range result.details {
		u.PrintIndentedSuccess(detail)
		lineCount++
	}

	// Print command phase
	cmdLines, _ := formatCommand(result.args)
	u.PrintInfo("Command")
	lineCount++
	for _, line := range cmdLines {
		u.PrintGeneric(line)
		lineCount++
	}

	// Run ffmpeg
	ffmpegArgs := append(result.args, "-progress", "pipe:1", "-nostats", "-loglevel", "error", "-y")
	cmd := exec.Command("ffmpeg", ffmpegArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	done := make(chan struct{})
	var currentPercent atomic.Int32
	var printed atomic.Bool
	outputName := filepath.Base(result.outputFile)

	// +1 for the progress line itself when clearing
	totalClearLines := lineCount + 1

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		firstTick := true
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				cmd.Process.Kill()
				return
			case <-ticker.C:
				if !firstTick {
					u.ClearPreviousLine()
				}
				firstTick = false
				printed.Store(true)
				u.PrintProgress(outputName, int(currentPercent.Load()))
			}
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "=")
		if len(parts) == 2 && parts[0] == "out_time_us" {
			currentUs, _ := strconv.ParseFloat(parts[1], 64)
			if totalDurationSecs > 0 {
				pct := int((currentUs / 1000000.0 / totalDurationSecs) * 100)
				if pct > 100 {
					pct = 100
				}
				currentPercent.Store(int32(pct))
			}
		}
	}

	cmdErr := cmd.Wait()
	close(done)

	// Clear all output: video info + command + progress line
	if printed.Load() {
		u.ClearLines(totalClearLines)
	} else {
		// Progress never printed, just clear video info + command (no progress line)
		u.ClearLines(lineCount)
	}

	stderrContent := strings.TrimSpace(stderrBuf.String())
	if cmdErr != nil {
		if stderrContent != "" {
			return fmt.Errorf("%s: %w", stderrContent, cmdErr)
		}
		return fmt.Errorf("ffmpeg encoding failed: %w", cmdErr)
	}

	fileInfo, _ := os.Stat(result.outputFile)
	sizeStr := ""
	if fileInfo != nil {
		sizeMB := float64(fileInfo.Size()) / 1024 / 1024
		if sizeMB >= 1024 {
			sizeStr = fmt.Sprintf(" (%.2f GB)", sizeMB/1024)
		} else {
			sizeStr = fmt.Sprintf(" (%.1f MB)", sizeMB)
		}
	}

	u.PrintSuccess(fmt.Sprintf("%s: encoded in %s%s", outputName, time.Since(startTime).Truncate(time.Second), sizeStr))
	return nil
}
