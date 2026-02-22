package server

import (
	"encoding/json"
	"fmt"
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
		plog().Info().Str("codec", videoCodec).Str("file", targetFile).Msg("video codec not HLS-compatible, will transcode to H.264")
	} else {
		plog().Info().Str("codec", videoCodec).Str("file", targetFile).Msg("video codec HLS-compatible, will copy")
	}

	audioTracks := media.GetAudioTracks(fullPath)
	selectedAudio := media.SelectBestAudioTrack(audioTracks)

	var audioArgs []string
	if selectedAudio != nil {
		audioArgs = []string{"-map", "0:v:0", "-map", fmt.Sprintf("0:%d", selectedAudio.Index)}
		plog().Info().
			Int("track", selectedAudio.Index).
			Str("codec", selectedAudio.Codec).
			Str("lang", selectedAudio.Language).
			Int("channels", selectedAudio.Channels).
			Str("file", targetFile).
			Msg("selected audio track")

		needsAudioTranscode := !media.IsAudioCompatible(selectedAudio.Codec) || selectedAudio.Channels > 2

		if needsAudioTranscode {
			audioArgs = append(audioArgs, "-c:a", "aac", "-b:a", "192k", "-ac", "2", "-ar", "48000")
			if selectedAudio.Channels > 2 {
				plog().Info().Int("channels", selectedAudio.Channels).Msg("downmixing to stereo for browser compatibility")
			} else {
				plog().Info().Str("codec", selectedAudio.Codec).Msg("audio codec not compatible, transcoding to AAC")
			}
		} else {
			audioArgs = append(audioArgs, "-c:a", "copy")
		}
	} else {
		plog().Info().Str("file", targetFile).Msg("no audio tracks found")
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
			plog().Error().Err(err).Str("path", subPath).Msg("failed to convert external subtitle")
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
			plog().Error().Err(err).Int("track", track.Index).Msg("failed to extract embedded subtitle")
		}
	}

	plog().Info().Int("count", len(subtitleList)).Str("session", sessionID).Msg("subtitles found")

	playlistPath := filepath.Join(sessionDir, "index.m3u8")
	segmentPath := filepath.Join(sessionDir, "seg_%03d.ts")

	const segmentDuration = 6.0
	if err := media.GenerateHLSPlaylist(playlistPath, duration, segmentDuration); err != nil {
		plog().Error().Err(err).Msg("failed to generate playlist")
		http.Error(w, "Failed to generate playlist", 500)
		return
	}
	plog().Info().
		Str("session", sessionID).
		Float64("duration", duration).
		Int("segments", int(duration/segmentDuration)+1).
		Msg("pre-generated HLS playlist")

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
		plog().Error().Err(err).Msg("failed to start ffmpeg")
		http.Error(w, "Failed to start stream", 500)
		return
	}

	plog().Info().Str("session", sessionID).Str("file", targetFile).Msg("started HLS stream")

	s.streamMutex.Lock()
	s.activeStreams[sessionID] = cmd
	s.streamMutex.Unlock()

	firstSeg := filepath.Join(sessionDir, "seg_000.ts")
	firstSegReady := media.WaitForFile(firstSeg, 50, 200*time.Millisecond)
	if !firstSegReady {
		plog().Warn().Msg("HLS not ready: first segment not available")
		http.Error(w, "Stream not ready", http.StatusServiceUnavailable)
		return
	}

	plog().Info().Str("session", sessionID).Msg("HLS ready")

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
		plog().Info().Str("session", sessionID).Msg("stopped HLS stream")
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
			plog().Warn().Str("path", fullPath).Msg("HLS rejected traversal")
			http.NotFound(w, r)
			return
		}
		if _, err := os.Stat(fullPath); err != nil {
			plog().Debug().Str("path", fullPath).Err(err).Msg("HLS miss")
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
