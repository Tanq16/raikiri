package media

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// GenerateHLSPlaylist creates a VOD m3u8 playlist file based on video duration.
func GenerateHLSPlaylist(playlistPath string, duration float64, segmentDuration float64) error {
	numSegments := int(duration / segmentDuration)
	lastSegmentDuration := duration - (float64(numSegments) * segmentDuration)

	if lastSegmentDuration > 0 {
		numSegments++
	} else {
		lastSegmentDuration = segmentDuration
	}

	var content strings.Builder
	content.WriteString("#EXTM3U\n")
	content.WriteString("#EXT-X-VERSION:3\n")
	content.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", int(segmentDuration)+1))
	content.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	content.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")

	for i := 0; i < numSegments; i++ {
		segDur := segmentDuration
		if i == numSegments-1 {
			segDur = lastSegmentDuration
		}
		content.WriteString(fmt.Sprintf("#EXTINF:%.6f,\n", segDur))
		content.WriteString(fmt.Sprintf("seg_%03d.ts\n", i))
	}

	content.WriteString("#EXT-X-ENDLIST\n")

	return os.WriteFile(playlistPath, []byte(content.String()), 0644)
}

// WaitForFile waits for a file to exist and be non-empty.
func WaitForFile(path string, attempts int, sleep time.Duration) bool {
	for i := 0; i < attempts; i++ {
		info, err := os.Stat(path)
		if err == nil && info.Size() > 0 {
			return true
		}
		time.Sleep(sleep)
	}
	return false
}
