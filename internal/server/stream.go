package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tanq16/raikiri/internal/media"
)

// HandleStreamStart creates an HLS stream session for a video file.
func (s *Server) HandleStreamStart(w http.ResponseWriter, r *http.Request) {
	targetFile := r.URL.Query().Get("file")
	mode := r.URL.Query().Get("mode")
	root := s.getRoot(mode)
	fullPath := filepath.Join(root, targetFile)

	duration, err := media.GetVideoDuration(fullPath)
	if err != nil {
		http.Error(w, "Failed to get video duration", 500)
		return
	}

	videoCodec := media.GetVideoCodec(fullPath)
	needsVideoTranscode := !media.IsVideoCompatibleForHLS(videoCodec)
	if needsVideoTranscode {
		log.Printf("INFO [server] video codec not HLS-compatible, will transcode to H.264 codec=%s file=%s", videoCodec, targetFile)
	} else {
		log.Printf("INFO [server] video codec HLS-compatible, will copy codec=%s file=%s", videoCodec, targetFile)
	}

	audioTracks := media.GetAudioTracks(fullPath)
	selectedAudio := media.SelectBestAudioTrack(audioTracks)

	var audioArgs []string
	if selectedAudio != nil {
		audioArgs = []string{"-map", "0:v:0", "-map", fmt.Sprintf("0:%d", selectedAudio.Index)}
		log.Printf("INFO [server] selected audio track track=%d codec=%s lang=%s channels=%d file=%s", selectedAudio.Index, selectedAudio.Codec, selectedAudio.Language, selectedAudio.Channels, targetFile)

		needsAudioTranscode := !media.IsAudioCompatible(selectedAudio.Codec) || selectedAudio.Channels > 2

		if needsAudioTranscode {
			audioArgs = append(audioArgs, "-c:a", "aac", "-b:a", "192k", "-ac", "2", "-ar", "48000")
			if selectedAudio.Channels > 2 {
				log.Printf("INFO [server] downmixing to stereo for browser compatibility channels=%d", selectedAudio.Channels)
			} else {
				log.Printf("INFO [server] audio codec not compatible, transcoding to AAC codec=%s", selectedAudio.Codec)
			}
		} else {
			audioArgs = append(audioArgs, "-c:a", "copy")
		}
	} else {
		log.Printf("INFO [server] no audio tracks found file=%s", targetFile)
		audioArgs = []string{"-map", "0:v:0"}
	}

	sessionID := fmt.Sprintf("s_%d", time.Now().UnixNano())
	sessionDir := filepath.Join(s.config.CachePath, sessionID)
	os.MkdirAll(sessionDir, 0755)

	var subtitleList []map[string]interface{}
	subtitleCounter := 1

	externalSubs := media.FindExternalSubtitles(fullPath)
	for _, subPath := range externalSubs {
		dstPath := filepath.Join(sessionDir, fmt.Sprintf("sub_%d.vtt", subtitleCounter))
		if err := media.ConvertSRTtoVTT(subPath, dstPath); err == nil {
			subtitleList = append(subtitleList, map[string]interface{}{
				"index": subtitleCounter,
				"label": fmt.Sprintf("Sub %d", subtitleCounter),
			})
			subtitleCounter++
		} else {
			log.Printf("ERROR [server] failed to convert external subtitle path=%s: %v", subPath, err)
		}
	}

	embeddedSubs := media.GetEmbeddedSubtitleTracks(fullPath)
	for _, track := range embeddedSubs {
		dstPath := filepath.Join(sessionDir, fmt.Sprintf("sub_%d.vtt", subtitleCounter))
		if err := media.ExtractSubtitleToSRT(fullPath, track.Index, dstPath); err == nil {
			subtitleList = append(subtitleList, map[string]interface{}{
				"index": subtitleCounter,
				"label": fmt.Sprintf("Sub %d", subtitleCounter),
			})
			subtitleCounter++
		} else {
			log.Printf("ERROR [server] failed to extract embedded subtitle track=%d: %v", track.Index, err)
		}
	}

	log.Printf("INFO [server] subtitles found count=%d session=%s", len(subtitleList), sessionID)

	playlistPath := filepath.Join(sessionDir, "index.m3u8")
	segmentPath := filepath.Join(sessionDir, "seg_%03d.ts")

	const segmentDuration = 6.0
	if err := media.GenerateHLSPlaylist(playlistPath, duration, segmentDuration); err != nil {
		log.Printf("ERROR [server] failed to generate playlist: %v", err)
		http.Error(w, "Failed to generate playlist", 500)
		return
	}
	log.Printf("INFO [server] pre-generated HLS playlist session=%s duration=%.2f segments=%d", sessionID, duration, int(duration/segmentDuration)+1)

	args := []string{
		"-loglevel", "warning",
		"-i", fullPath,
	}
	args = append(args, audioArgs...)
	if needsVideoTranscode {
		args = append(args,
			"-c:v", "libx264",
			"-preset", "fast",
			"-crf", "23",
			"-maxrate", "3000k",
			"-bufsize", "6000k",
		)
	} else {
		args = append(args, "-c:v", "copy")
	}

	args = append(args,
		"-f", "hls",
		"-hls_time", "6",
		"-hls_list_size", "0",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", segmentPath,
		playlistPath,
	)

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Printf("ERROR [server] failed to start ffmpeg: %v", err)
		http.Error(w, "Failed to start stream", 500)
		return
	}

	log.Printf("INFO [server] started HLS stream session=%s file=%s", sessionID, targetFile)

	s.streamMutex.Lock()
	s.activeStreams[sessionID] = cmd
	s.streamMutex.Unlock()

	firstSeg := filepath.Join(sessionDir, "seg_000.ts")
	firstSegReady := media.WaitForFile(firstSeg, 50, 200*time.Millisecond)
	if !firstSegReady {
		log.Printf("WARN [server] HLS not ready: first segment not available")
		http.Error(w, "Stream not ready", http.StatusServiceUnavailable)
		return
	}

	log.Printf("INFO [server] HLS ready session=%s", sessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"url":       fmt.Sprintf("/api/hls/%s/index.m3u8", sessionID),
		"altUrl":    fmt.Sprintf("/hls/%s/index.m3u8", sessionID),
		"sessionId": sessionID,
		"duration":  duration,
		"subtitles": subtitleList,
	})
}

// HandleStreamStop kills an active ffmpeg session and cleans up.
func (s *Server) HandleStreamStop(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		return
	}

	s.streamMutex.Lock()
	if cmd, exists := s.activeStreams[sessionID]; exists {
		cmd.Process.Kill()
		cmd.Wait()
		delete(s.activeStreams, sessionID)
		log.Printf("INFO [server] stopped HLS stream session=%s", sessionID)
		go os.RemoveAll(filepath.Join(s.config.CachePath, sessionID))
	}
	s.streamMutex.Unlock()

	w.WriteHeader(200)
}

// makeHLSHandler serves files from the HLS cache directory.
func (s *Server) makeHLSHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/")
		rel = filepath.Clean(rel)
		fullPath := filepath.Join(s.config.CachePath, rel)
		if !strings.HasPrefix(fullPath, s.config.CachePath) {
			log.Printf("WARN [server] HLS rejected traversal path=%s", fullPath)
			http.NotFound(w, r)
			return
		}
		if _, err := os.Stat(fullPath); err != nil {
			log.Printf("DEBUG [server] HLS miss path=%s: %v", fullPath, err)
			http.NotFound(w, r)
			return
		}

		if strings.HasSuffix(fullPath, ".srt") || strings.HasSuffix(fullPath, ".vtt") {
			w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		http.ServeFile(w, r, fullPath)
	})
}
