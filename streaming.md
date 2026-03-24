# Streaming & Encoding — Technical Reference

Everything learned from testing, debugging, and building the raikiri media server and video encoding pipeline.

---

## Video Encoding (ffmpeg)

### Core Encoding Command

```bash
ffmpeg -i input \
  -c:v libx265 -crf {crf} -preset slow -fps_mode cfr \
  [-r {target_fps}]              # only for standard fps halving
  -tag:v hvc1 \                  # Safari/iOS HEVC compatibility
  -c:a aac -ac 2 -ar 48000 -b:a 192k \
  -avoid_negative_ts make_zero \
  -movflags +faststart \         # moov atom at front for HTTP streaming
  output.mp4
```

### Key Flags

| Flag | Purpose |
|------|---------|
| `-fps_mode cfr` | Forces constant frame rate output. Without it, ffmpeg defaults to `auto` which can produce VFR from VFR sources. VFR causes inconsistent HLS segment durations and confuses browser decoders. |
| `-tag:v hvc1` | Marks HEVC as `hvc1` instead of `hev1`. Required for Safari/iOS native playback and direct serve on Apple devices. |
| `-movflags +faststart` | Moves the moov atom to the front of the MP4 file. Critical for HTTP range-request serving — without it, the browser has to download the end of the file before it can start playing. |
| `-avoid_negative_ts make_zero` | Shifts negative timestamps to zero. Prevents issues with sources that have negative DTS/PTS at the start. |
| `-ar 48000` | Forces 48kHz audio sample rate. Browser standard; 44.1kHz sources get resampled. |
| `-ac 2` | Downmix to stereo. Browsers don't reliably handle surround audio. |
| `-b:a 192k` | AAC at 192kbps stereo is perceptually transparent for virtually all material. |

### Frame Rate Handling

Only standard high frame rates are auto-halved to avoid introducing non-standard rates that cause timestamp arithmetic drift:

| Source | Target | Rational Form |
|--------|--------|--------------|
| 60 fps | 30 fps | 60/1 → 30/1 |
| 59.94 fps | 29.97 fps | 60000/1001 → 30000/1001 |
| 50 fps | 25 fps | 50/1 → 25/1 |
| 48 fps | 24 fps | 48/1 → 24/1 |
| 47.95 fps | 23.976 fps | 48000/1001 → 24000/1001 |

Non-standard rates (e.g., 24.83 fps) are left untouched — halving them would create irrational PTS intervals that accumulate rounding errors.

The halving is done by doubling the denominator of the rational frame rate (preserves precision), then simplifying via GCD.

### 10-bit Color

libx265 auto-detects the input pixel format. If the source is 10-bit (`yuv420p10le`), the output will be 10-bit — no special flag needed. Explicitly passing `-pix_fmt yuv420p10le` ensures it's retained even if ffmpeg's auto-detection gets confused.

### Preset: slow vs medium

Preset controls encode speed vs compression efficiency, NOT quality. CRF alone controls quality. A CRF 24 with `slow` looks the same as CRF 24 with `medium` — just produces a smaller file. Since encoding is offline (not real-time), `slow` is the default. The `--faster` flag switches to `medium` for quick encodes when needed.

### Quality Tiers

| Tier | CRF | Use Case |
|------|-----|----------|
| very-high | 22 | Archival, reference quality |
| high | 24 | High quality, reasonable size |
| medium | 26 | Default, good balance |
| low | 28 | Space-constrained, still decent |

### Audio Stream Selection

1. Reject commentary tracks (regex: `commentary|director|cast`), visually impaired, and comment-disposition streams
2. Prefer English (`eng` or empty language tag)
3. Fall back to first non-rejected stream
4. Last resort: first stream regardless

### Subtitle Handling

- Text-based codecs (`subrip`, `ass`, `ssa`, `webvtt`, `mov_text`, `srt`) → MP4 container with `mov_text` codec
- Bitmap codecs (`hdmv_pgs_subtitle`, `vobsub`, `dvd_subtitle`) → MKV container with codec copy (MP4 can't hold bitmap subs)
- Output extension chosen automatically: `.h265.mp4` or `.h265.mkv`

---

## A/V Drift in Browsers

### The Problem

Progressive audio-video desynchronization in browser playback. VLC plays everything perfectly (it has real-time clock correction via `swr_set_compensation()`). Browsers lack this capability — they rely on the source timestamps being correct.

### Root Cause

Source files often have subtle timestamp irregularities in audio streams. When ffmpeg re-encodes, it derives output timestamps from input timestamps. These irregularities pass through even with audio re-encoding (without aresample). Over long playback durations, the cumulative error causes visible drift.

### What Fixes Drift

The ONLY proven server-side fix is `aresample=osr=48000:first_pts=0`. This:
1. Anchors the first audio PTS at exactly 0
2. Enforces strict alignment between timestamps and cumulative sample count
3. Inserts silence or drops samples when it detects gaps/overlaps (hard compensation)

The trade-off: audible hiccups approximately every ~20 seconds where hard corrections occur. These hiccups happen even on 48kHz→48kHz passthrough (nits-encoded files already at 48kHz), meaning the source timestamps have significant irregularities, not just slow drift.

### What Does NOT Fix Drift

Extensively tested (26+ encoding variants, 5 HLS configurations):

- `-c:a copy` (preserves bad timestamps)
- `-c:a aac -ar 48000` without aresample (derives timestamps from source)
- `-fflags +genpts` (regenerates from DTS, still inherits source timing)
- `-asetpts=PTS-STARTPTS` (resets start but preserves intervals)
- `-asetpts=NB_CONSUMED_SAMPLES` (pure rebuild, still drifts)
- `-copyts` / `-avoid_negative_ts disabled` (Jellyfin approach)
- `-video_track_timescale 90000`
- Two-step encode via intermediate FLAC
- Video transcoding to H.264 (drift is in audio, not video)
- `aresample=async=1000` (soft compensation only — stretches but insufficient for significant irregularities)
- `aresample=osr=48000:first_pts=0:min_hard_comp=9999` (disable hard comp → drift returns)
- `aresample=osr=48000,asetpts=NB_CONSUMED_SAMPLES` (timestamp rebuild → drift)

### aresample Variants Tested

| Variant | Result |
|---------|--------|
| `aresample=osr=48000:first_pts=0` | No drift, audible hiccups ~20s |
| `aresample=osr=48000:first_pts=0:min_hard_comp=0.003` | No drift, hiccups still perceptible |
| `aresample=osr=48000:first_pts=0:resampler=soxr` | No drift, worse stuttering |
| `aresample=async=1000:first_pts=0:min_hard_comp=0.5` | Errors, lost audio, gross misplacement |
| `aresample=osr=48000:first_pts=0:async=1000:min_hard_comp=9999` | No drift, pitch modulation artifacts |
| `aresample=osr=48000:first_pts=0:min_hard_comp=100:min_comp=100` | Drift returns (hard comp effectively disabled) |

### Drift Detection — All Approaches Failed

1. **Stream duration comparison** — Compare audio vs video stream durations. Failed because 16.mp4 has 984ms gap (385 ppm) but no drift, while 2x06.mp4 has only 28ms gap (19.2 ppm) and drifts badly. No reliable threshold exists.

2. **Audio packet PTS spacing** — Check inter-packet timing regularity. Failed because output.mp4 shows perfect spacing but still drifts. Local packet regularity doesn't predict cross-stream alignment.

3. **Cross-stream PTS comparison** — Compare audio PTS vs video PTS at multiple seek points. Failed because ffprobe `-read_intervals` seeks to nearest keyframe for video but nearest packet for audio — different temporal positions. Shows 3600ms/hour drift on files with zero actual drift.

---

## HLS Streaming

### Segment Types: fMP4 vs MPEG-TS

This is the most important architectural discovery from testing.

#### fMP4 (Fragmented MP4)

```
-hls_segment_type fmp4
-hls_fmp4_init_filename init.mp4
-hls_segment_filename seg_%03d.m4s
```

- Produces an init segment (`init.mp4`) + data segments (`.m4s`)
- HLS.js uses **PassthroughRemuxer** for fMP4 → passes data straight through with zero per-frame processing
- `maxAudioFramesDrift` and all frame-level A/V correction logic is **completely inactive** for fMP4
- Drift in the source passes straight through to the browser
- Lighter on the client (no demux/remux needed)

#### MPEG-TS

```
-hls_segment_type mpegts
-hls_segment_filename seg_%03d.ts
```

- Self-contained segments (no init segment needed)
- HLS.js uses **TSDemuxer → MP4Remuxer** pipeline
- The MP4Remuxer has **frame-level drift correction** via `maxAudioFramesDrift` (default: 1 frame ≈ 21ms at 48kHz)
- Silently inserts silent AAC frames when audio gaps are detected
- Drops overlapping audio frames
- This correction is AAC-specific and only active when audio is aligned with video (`alignedWithVideo = true`)
- Heavier on the client (full demux + remux per segment)

#### Key Insight

All HLS tests using fMP4 drifted. Switching to MPEG-TS fixed drift via HLS.js's client-side correction. The minor hiccups from HLS.js inserting/dropping frames are much more tolerable than aresample's hard compensation hiccups.

### HLS.js Architecture

#### Two Remuxer Paths

```
fMP4 input  → MP4Demuxer    → PassthroughRemuxer  → SourceBuffer (no correction)
MPEG-TS     → TSDemuxer     → MP4Remuxer          → SourceBuffer (drift correction)
```

The transmuxer probes input data format and selects the path automatically. There is no configuration to force MP4Remuxer for fMP4 — it's architecturally determined by segment type.

#### Drift Correction in MP4Remuxer (MPEG-TS only)

In `remuxAudio()`:
- Computes `delta = pts - nextPts` for each audio frame
- **Frame dropping**: When `delta <= -maxAudioFramesDrift * inputSampleDuration && alignedWithVideo`, the frame is dropped
- **Silent frame insertion**: When `delta >= maxAudioFramesDrift * inputSampleDuration && duration < 10s && alignedWithVideo`, inserts `Math.round(delta / inputSampleDuration)` silent AAC frames via `AAC.getSilentFrame()`
- Silent frames are pre-baked byte arrays for AAC-LC (1-6 channels) and HE-AAC (1-3 channels)

#### HLS.js Sync Layers

1. **InitPTS sharing** — StreamController fires `INIT_PTS_FOUND`, AudioStreamController stores it per continuity counter
2. **Fragment-level timestamp alignment** — Asymmetric offset adjustment in `MP4Remuxer.remux()` based on audio-video PTS delta
3. **Sample-level drift correction** — Frame dropping/insertion per `maxAudioFramesDrift` (MPEG-TS only)
4. **Gap Controller** — Runtime stall detection, progressive nudges (`currentTime += retryCount * nudgeOffset`)

#### Relevant HLS.js Config

| Config | Default | Purpose |
|--------|---------|---------|
| `maxAudioFramesDrift` | 1 | Frames of drift before correction (1 frame ≈ 21ms at 48kHz). AAC only, MPEG-TS only. |
| `stretchShortVideoTrack` | false | Extends last video frame duration to match audio length |
| `maxBufferHole` | 0.1s | Maximum acceptable gap in buffered media |
| `nudgeOffset` | 0.1s | Seek increment per stall recovery retry |
| `nudgeMaxRetry` | 3 | Maximum nudge attempts before fatal error |
| `startPosition` | -1 | Force start position. Set to 0 for VOD to prevent seeking to live edge. |
| `backBufferLength` | Infinity | Past content retention |
| `maxMaxBufferLength` | 600s | Hard ceiling for buffer |

### Raikiri's HLS Configuration

```javascript
new Hls({
    enableWorker: true,
    lowLatencyMode: false,
    startPosition: 0,              // force start at segment 0
    stretchShortVideoTrack: true,  // prevent stalls at segment ends
    backBufferLength: 60,
    maxMaxBufferLength: 120,
    nudgeMaxRetry: 5,
    manifestLoadingMaxRetry: 2,
});
```

### Audio Handling per Source Type

| Source | Audio Strategy | Rationale |
|--------|---------------|-----------|
| Direct serve | No processing | Raw file, browser handles natively |
| HLS fMP4 | `aresample=osr=48000:first_pts=0` | PassthroughRemuxer does no correction — must fix server-side |
| HLS MPEG-TS | Copy if compatible, else basic re-encode (no aresample) | MP4Remuxer handles drift correction client-side |

---

## Serving Architecture

### Source Priority (increasing server load)

1. **Direct Serve** — Zero CPU. HTTP range-request file serving. No ffmpeg process. Requires: MP4/MOV container, H.264/HEVC video, AAC/MP3/Opus audio, stereo, 48kHz.

2. **HLS fMP4** — Light. Usually `-c:v copy` (no video transcode). Audio re-encoded with aresample for drift compensation. fMP4 muxing is lightweight.

3. **HLS MPEG-TS** — Same server load as fMP4 (muxing cost difference is negligible). The heavier part is **client-side**: HLS.js demuxes every TS segment, runs it through MP4Remuxer (drift correction), and re-muxes to fMP4 for the browser's MSE.

### Source Cycling

The frontend cycles through sources on button press or auto-fallback:

- `direct` → `hls-fmp4` → `hls-ts` → (wrap)
- Auto-fallback: direct play failure → next source
- Preserves playback position via `loadeddata` event listener
- Each source request creates a new server session (ffmpeg process)
- Previous session is stopped and cleaned up

### Direct Serve Criteria

All must be true:
- Container: MP4 or MOV
- Video codec: H.264, AVC, HEVC, or H.265
- Audio codec: AAC, MP3, or Opus
- Audio channels: ≤ 2 (mono/stereo)
- Audio sample rate: exactly 48kHz

Files encoded with the raikiri `video-encode` command always pass these checks (MP4 + HEVC hvc1 + AAC stereo 48kHz).

### HLS Segment Settings

```
-f hls
-hls_time 6                    # 6-second segments
-hls_list_size 0               # keep all segments in manifest
-hls_playlist_type event       # VOD-like, finite duration
-max_interleave_delta 0
-max_muxing_queue_size 4096
-start_at_zero
-avoid_negative_ts make_zero
```

### MIME Types

| Extension | MIME Type |
|-----------|-----------|
| `.m3u8` | `application/vnd.apple.mpegurl` (served by Go's http.ServeFile) |
| `.m4s` | `video/iso.segment` |
| `.mp4` | `video/mp4` |
| `.ts` | `video/mp2t` |
| `.vtt` | `text/vtt; charset=utf-8` |

---

## Jellyfin Comparison

Investigated Jellyfin's approach for reference:

- Uses stock HLS.js (1.6.15) with minimal config changes (smaller buffers for high bitrate)
- Hardcodes `-copyts -avoid_negative_ts disabled` for ALL HLS
- Never uses `aresample` — just simple `-ar` for rate conversion
- Has **94 custom ffmpeg patches**, including a segment muxer fix that anchors segments to actual stream start PTS
- Does NOT normalize frame rates
- Does NOT set `maxAudioFramesDrift`, `stretchShortVideoTrack`, or other HLS.js sync options

Jellyfin's `-copyts` approach was tested (HLS test 2) and did NOT fix drift in our case. Their approach likely works because of the custom ffmpeg patches rather than the timestamp flags alone.

---

## Browser vs Desktop Player Behavior

| Capability | VLC/Kodi | Browser |
|-----------|----------|---------|
| Real-time clock correction | Yes (`swr_set_compensation()`) | No |
| Audio speed micro-adjustment | Yes (< 0.1% changes) | No |
| Drift tolerance | Handles any drift transparently | Accumulates until visible |
| DTS-based buffering | No (uses PTS) | Chrome buffers by DTS, can cause misalignment |
| AAC encoder delay handling | Consistent | Varies across browsers |

This is why VLC plays everything perfectly while browsers drift — they're fundamentally different playback architectures.
