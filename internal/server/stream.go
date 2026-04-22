package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tanq16/raikiri/internal/media"
)

func extractSubtitles(fullPath, sessionDir string) []map[string]interface{} {
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

	return subtitleList
}

func (s *Server) HandleStreamStart(w http.ResponseWriter, r *http.Request) {
	targetFile := r.URL.Query().Get("file")
	mode := r.URL.Query().Get("mode")
	source := r.URL.Query().Get("source") // "direct", "remux", "hls-fmp4", "hls-ts", or "" (auto)
	forceHLS := r.URL.Query().Get("force") == "hls"
	root := s.getRoot(mode)
	fullPath := filepath.Join(root, targetFile)

	duration, err := media.GetVideoDuration(fullPath)
	if err != nil {
		http.Error(w, "Failed to get video duration", 500)
		return
	}

	sessionID := fmt.Sprintf("s_%d", time.Now().UnixNano())
	sessionDir := filepath.Join(s.config.CachePath, sessionID)
	os.MkdirAll(sessionDir, 0755)

	subtitleList := extractSubtitles(fullPath, sessionDir)
	log.Printf("INFO [server] subtitles found count=%d session=%s", len(subtitleList), sessionID)

	isServable := media.IsDirectServable(fullPath)
	videoCodec := media.GetVideoCodec(fullPath)
	canRemux := media.IsVideoCompatibleForHLS(videoCodec)

	availableSources := []string{}
	if isServable {
		availableSources = append(availableSources, "direct")
	}
	if canRemux {
		availableSources = append(availableSources, "remux")
	}
	availableSources = append(availableSources, "hls-fmp4", "hls-ts")

	if source == "" {
		if forceHLS {
			if canRemux {
				source = "remux"
			} else {
				source = "hls-fmp4"
			}
		} else if isServable {
			source = "direct"
		} else if canRemux {
			source = "remux"
		} else {
			source = "hls-fmp4"
		}
	}

	if source == "direct" && !isServable {
		if canRemux {
			source = "remux"
		} else {
			source = "hls-fmp4"
		}
	}
	if source == "remux" && !canRemux {
		source = "hls-fmp4"
	}

	if source == "direct" {
		log.Printf("INFO [server] direct serve file=%s", targetFile)

		if len(subtitleList) == 0 {
			os.RemoveAll(sessionDir)
			sessionID = ""
		}

		segments := strings.Split(targetFile, "/")
		for i, seg := range segments {
			segments[i] = url.PathEscape(seg)
		}
		contentURL := fmt.Sprintf("/content/%s?mode=%s", strings.Join(segments, "/"), mode)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mode":             "direct",
			"source":           "direct",
			"url":              contentURL,
			"duration":         duration,
			"sessionId":        sessionID,
			"subtitles":        subtitleList,
			"availableSources": availableSources,
		})
		return
	}

	isRemux := source == "remux"
	isTS := source == "hls-ts"

	needsVideoTranscode := !media.IsVideoCompatibleForHLS(videoCodec)
	if isRemux {
		log.Printf("INFO [server] remux mode: copying video codec=%s file=%s", videoCodec, targetFile)
	} else if needsVideoTranscode {
		log.Printf("INFO [server] video codec not HLS-compatible, will transcode to H.264 codec=%s file=%s", videoCodec, targetFile)
	} else {
		log.Printf("INFO [server] video codec HLS-compatible, will copy codec=%s file=%s", videoCodec, targetFile)
	}

	audioTracks := media.GetAudioTracks(fullPath)
	selectedAudio := media.SelectBestAudioTrack(audioTracks)

	var audioArgs []string
	if selectedAudio != nil {
		audioArgs = []string{"-map", "0:v:0", "-map", fmt.Sprintf("0:%d", selectedAudio.Index)}
		log.Printf("INFO [server] selected audio track=%d codec=%s lang=%s channels=%d file=%s", selectedAudio.Index, selectedAudio.Codec, selectedAudio.Language, selectedAudio.Channels, targetFile)

		if isRemux {
			canCopyAudio := selectedAudio.Codec == "aac" && selectedAudio.Channels <= 2
			if canCopyAudio {
				log.Printf("INFO [server] remux: copying compatible audio file=%s", targetFile)
				audioArgs = append(audioArgs, "-c:a", "copy")
			} else {
				log.Printf("INFO [server] remux: re-encoding audio to AAC stereo file=%s", targetFile)
				audioArgs = append(audioArgs, "-c:a", "aac", "-b:a", "192k", "-ac", "2", "-ar", "48000")
			}
		} else if isTS {
			needsAudioTranscode := !media.IsAudioCompatible(selectedAudio.Codec) || selectedAudio.Channels > 2
			sampleRate := media.GetAudioSampleRate(fullPath, selectedAudio.Index)
			if needsAudioTranscode || sampleRate != 48000 {
				log.Printf("INFO [server] HLS-TS: re-encoding audio to AAC 48kHz stereo file=%s", targetFile)
				audioArgs = append(audioArgs, "-c:a", "aac", "-b:a", "192k", "-ac", "2", "-ar", "48000")
			} else {
				log.Printf("INFO [server] HLS-TS: copying compatible audio file=%s", targetFile)
				audioArgs = append(audioArgs, "-c:a", "copy")
			}
		} else {
			log.Printf("INFO [server] HLS-fMP4: re-encoding audio with aresample file=%s", targetFile)
			audioArgs = append(audioArgs, "-c:a", "aac", "-b:a", "192k", "-ac", "2", "-af", "aresample=osr=48000:first_pts=0")
		}
	} else {
		log.Printf("INFO [server] no audio tracks found file=%s", targetFile)
		audioArgs = []string{"-map", "0:v:0"}
	}

	playlistPath := filepath.Join(sessionDir, "index.m3u8")

	args := []string{
		"-loglevel", "warning",
		"-start_at_zero",
		"-i", fullPath,
	}
	args = append(args, audioArgs...)
	if isRemux || !needsVideoTranscode {
		args = append(args, "-c:v", "copy")
	} else {
		args = append(args,
			"-c:v", "libx264",
			"-preset", "fast",
			"-crf", "23",
		)
	}

	args = append(args,
		"-avoid_negative_ts", "make_zero",
		"-max_interleave_delta", "0",
		"-max_muxing_queue_size", "4096",
		"-f", "hls",
		"-hls_time", "6",
		"-hls_list_size", "0",
		"-hls_playlist_type", "event",
	)

	if isTS {
		segmentPath := filepath.Join(sessionDir, "seg_%03d.ts")
		args = append(args,
			"-hls_segment_type", "mpegts",
			"-hls_segment_filename", segmentPath,
		)
	} else {
		segmentPath := filepath.Join(sessionDir, "seg_%03d.m4s")
		args = append(args,
			"-hls_segment_type", "fmp4",
			"-hls_fmp4_init_filename", "init.mp4",
			"-hls_segment_filename", segmentPath,
		)
	}
	args = append(args, playlistPath)

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Printf("ERROR [server] failed to start ffmpeg: %v", err)
		http.Error(w, "Failed to start stream", 500)
		return
	}

	log.Printf("INFO [server] started HLS stream session=%s source=%s file=%s", sessionID, source, targetFile)

	s.streamMutex.Lock()
	s.activeStreams[sessionID] = cmd
	s.streamMutex.Unlock()

	var firstSegReady bool
	if isTS {
		firstSegReady = media.WaitForFile(filepath.Join(sessionDir, "seg_000.ts"), 50, 200*time.Millisecond) &&
			media.WaitForFile(playlistPath, 50, 200*time.Millisecond)
	} else {
		firstSegReady = media.WaitForFile(filepath.Join(sessionDir, "init.mp4"), 50, 200*time.Millisecond) &&
			media.WaitForFile(filepath.Join(sessionDir, "seg_000.m4s"), 50, 200*time.Millisecond) &&
			media.WaitForFile(playlistPath, 50, 200*time.Millisecond)
	}
	if !firstSegReady {
		log.Printf("INFO [server] HLS not ready, killing ffmpeg session=%s", sessionID)
		s.streamMutex.Lock()
		if cmd, exists := s.activeStreams[sessionID]; exists {
			cmd.Process.Kill()
			cmd.Wait()
			delete(s.activeStreams, sessionID)
		}
		s.streamMutex.Unlock()
		go func() {
			if err := os.RemoveAll(sessionDir); err != nil {
				log.Printf("ERROR [server] failed to remove session dir=%s: %v", sessionDir, err)
			}
		}()
		http.Error(w, "Stream not ready", http.StatusServiceUnavailable)
		return
	}

	log.Printf("INFO [server] HLS ready session=%s source=%s", sessionID, source)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mode":             "hls",
		"source":           source,
		"url":              fmt.Sprintf("/api/hls/%s/index.m3u8", sessionID),
		"altUrl":           fmt.Sprintf("/hls/%s/index.m3u8", sessionID),
		"sessionId":        sessionID,
		"duration":         duration,
		"subtitles":        subtitleList,
		"availableSources": availableSources,
	})
}

// HandleAudioFMP4 streams an audio file as fragmented MP4 suitable for direct
// MSE SourceBuffer.appendBuffer(). M4A (AAC) is remuxed (zero CPU); other
// formats are transcoded to AAC. The response is the raw fMP4 byte stream.
func (s *Server) HandleAudioFMP4(w http.ResponseWriter, r *http.Request) {
	targetFile := r.URL.Query().Get("file")
	mode := r.URL.Query().Get("mode")
	root := s.getRoot(mode)
	fullPath := filepath.Join(root, targetFile)

	if !strings.HasPrefix(fullPath, root) {
		http.NotFound(w, r)
		return
	}

	duration, err := media.GetAudioDuration(fullPath)
	if err != nil {
		http.Error(w, "Failed to get audio duration", 500)
		return
	}

	audioTracks := media.GetAudioTracks(fullPath)
	selectedAudio := media.SelectBestAudioTrack(audioTracks)

	args := []string{"-loglevel", "warning", "-i", fullPath, "-vn"}

	if selectedAudio != nil {
		args = append(args, "-map", fmt.Sprintf("0:%d", selectedAudio.Index))
		sampleRate := media.GetAudioSampleRate(fullPath, selectedAudio.Index)
		canCopy := selectedAudio.Codec == "aac" && selectedAudio.Profile == "LC" && selectedAudio.Channels <= 2 && sampleRate == 48000
		if canCopy {
			log.Printf("INFO [server] audio-fmp4: copying AAC-LC file=%s", targetFile)
			args = append(args, "-c:a", "copy")
		} else {
			log.Printf("INFO [server] audio-fmp4: transcoding to AAC-LC 48kHz codec=%s profile=%s rate=%d file=%s", selectedAudio.Codec, selectedAudio.Profile, sampleRate, targetFile)
			args = append(args, "-c:a", "aac", "-b:a", "192k", "-ac", "2", "-ar", "48000")
		}
	} else {
		args = append(args, "-c:a", "aac", "-b:a", "192k", "-ac", "2", "-ar", "48000")
	}

	args = append(args,
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"pipe:1",
	)

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, "Failed to create pipe", 500)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("ERROR [server] audio-fmp4 ffmpeg start failed: %v", err)
		http.Error(w, "Failed to start transcoder", 500)
		return
	}

	w.Header().Set("Content-Type", "audio/mp4")
	w.Header().Set("X-Audio-Duration", fmt.Sprintf("%.6f", duration))
	w.Header().Set("Cache-Control", "no-store")

	if _, err := io.Copy(w, stdout); err != nil {
		log.Printf("DEBUG [server] audio-fmp4 stream interrupted file=%s: %v", targetFile, err)
	}

	cmd.Wait()
}

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
	}
	s.streamMutex.Unlock()

	go func() {
		sessionDir := filepath.Join(s.config.CachePath, sessionID)
		if err := os.RemoveAll(sessionDir); err != nil {
			log.Printf("ERROR [server] failed to remove session dir=%s: %v", sessionDir, err)
		}
	}()

	w.WriteHeader(200)
}

func (s *Server) makeHLSHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/")
		rel = filepath.Clean(rel)
		fullPath := filepath.Join(s.config.CachePath, rel)
		if !strings.HasPrefix(fullPath, s.config.CachePath) {
			log.Printf("INFO [server] HLS rejected traversal path=%s", fullPath)
			http.NotFound(w, r)
			return
		}
		if _, err := os.Stat(fullPath); err != nil {
			log.Printf("DEBUG [server] HLS miss path=%s: %v", fullPath, err)
			http.NotFound(w, r)
			return
		}

		switch {
		case strings.HasSuffix(fullPath, ".m3u8"):
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		case strings.HasSuffix(fullPath, ".srt"), strings.HasSuffix(fullPath, ".vtt"):
			w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
			w.Header().Set("Access-Control-Allow-Origin", "*")
		case strings.HasSuffix(fullPath, ".m4s"):
			w.Header().Set("Content-Type", "video/iso.segment")
		case strings.HasSuffix(fullPath, ".mp4"):
			w.Header().Set("Content-Type", "video/mp4")
		case strings.HasSuffix(fullPath, ".ts"):
			w.Header().Set("Content-Type", "video/mp2t")
		}

		http.ServeFile(w, r, fullPath)
	})
}
