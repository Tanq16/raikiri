<div align="center">
  <img src=".github/assets/logo.png" alt="Local Content Share Logo" width="200">
  <h1>Raikiri</h1>

  <a href="https://github.com/tanq16/raikiri/actions/workflows/release.yml"><img alt="Build Workflow" src="https://github.com/tanq16/raikiri/actions/workflows/release.yml/badge.svg"></a>&nbsp;<a href="https://github.com/Tanq16/raikiri/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/tanq16/raikiri"></a>&nbsp;<a href="https://hub.docker.com/r/tanq16/raikiri"><img alt="Docker Pulls" src="https://img.shields.io/docker/pulls/tanq16/raikiri"></a><br><br>
</div>

A fast, simple, self-hosted, no-nonsense app for running a media server. This is meant for those instances when you don't need something beefy like Jellyfin or Plex, and don't want to go through the pain of metadata tagging.

The aim of the application is to provide directory listing in an elegant interface to view images and videos easily. There is no need for metadata and match, Raikiri just uses the folder navigation to display things. While Raikiri displays common image, video, and audio formats, other files will also be displayed to download directly.

## Features

- Beautiful Catppuccin Mocha themed application for modern web-based directory listing
- Dual system with separate Media and Music modes, each with independent directory paths; music navigation is folder-first (Artist → Album → Tracks) with no extra pills
- Tracks in Music are always shown in list view and skip thumbnail fetching
- Intelligent thumbnail system that displays preview images when available, with fallback to icons
- Playlist queue system that automatically creates playlists from media in the current directory; items can be removed from the queue
- Player bar with expanded player view supporting audio, video, and image playback
- Image slideshow mode with automatic advancement every 5 seconds
- Shuffle mode for recursive directory playback (media files only)
- Queue dialog showing current playlist with ability to jump to any item
- Fullscreen support for videos and images
- Subtitle support for videos with automatic detection of external SRT files and embedded subtitle tracks; switch between multiple subtitle tracks or disable subtitles
- Search functionality to filter files in the current directory
- Ability to upload files to the server at specific paths
- Functionality in the binary to prepare media for thumbnails (using `ffmpeg`)
- Automatic cache cleanup that removes old HLS session files older than 3 days
- Fully self-hosted with local assets and self-contained binary and container
- Efficient size for both binary and container - under 15 and 50 MB resp.

## Screenshots

<div align="center">

| | | |
|:---:|:---:|:---:|
| <img src=".github/assets/01.png" width="100%"> | <img src=".github/assets/02.png" width="100%"> | <img src=".github/assets/03.png" width="100%"> |
| <img src=".github/assets/04.png" width="100%"> | <img src=".github/assets/05.png" width="100%"> | <img src=".github/assets/06.png" width="100%"> |
| <img src=".github/assets/07.png" width="100%"> | <img src=".github/assets/08.png" width="100%"> | <img src=".github/assets/09.png" width="100%"> |

</div>

## Usage

### Docker (for Homelab)

```bash
mkdir $HOME/raikiri # you don't need to create this if you already have media in a specific directory
```
```bash
docker run --rm -d --name raikiri \
  -p 8080:8080 \
  -v $HOME/raikiri:/app/media \
  -v $HOME/music:/app/music \
  -v $HOME/raikiri-cache:/app/cache \
  tanq16/raikiri:main
```

The application will be available at `http://localhost:8080` (or your server IP). You can also use the following compose file:

```yaml
services:
  raikiri:
    image: tanq16/raikiri:main
    container_name: raikiri
    volumes:
      - /home/tanq/raikiri:/app/media # Change as needed
      - /home/tanq/music:/app/music # Change as needed
      - /home/tanq/raikiri-cache:/app/cache # HLS segment cache
    ports:
      - 8080:8080
```

### Binary

To use the binary, simply download the latest version from the project releases and run as follows:

```bash
raikiri -media $YOUR_MEDIA_FOLDER -music $YOUR_MUSIC_FOLDER
```

The `-media` flag specifies the path to your media directory (defaults to current directory), `-music` specifies the path to your music directory (defaults to `./music`), and `-cache` specifies the path to the cache directory for HLS segments (defaults to `/tmp`). You can switch between Media and Music modes using the tabs in the interface.

The `-cache` flag specified the cache directory, stores temporary HLS (HTTP Live Streaming) segments created during video transcoding (well, mostly transmuxing). These segments are generated on-the-fly when videos are played and are automatically cleaned up after playback ends. Raikiri also runs an automatic background cleanup process that removes cache sessions older than 3 days, running daily at 3 AM to keep the cache directory from growing indefinitely. While an SSD location is faster for performance, an HDD path is recommended for longevity since the cache involves frequent write operations during video playback. The default `/tmp` location is fine for most use cases, but you may want to specify a dedicated directory on a non-SSD drive for recurring use.

### Local development

With `Go 1.24+` installed, run the following to download the binary to your GOBIN:

```bash
go install github.com/tanq16/raikiri@latest
```

Or, you can build from source like so:

```bash
git clone https://github.com/tanq16/raikiri.git && \
cd raikiri && \
go build .
```

### Additional Notes

#### Requirements

Raikiri requires `ffmpeg` (which includes `ffprobe`) to be installed and available in your system's PATH for:
- Video playback: Videos are transcoded to HLS format on-the-fly
- Thumbnail generation: When using the `-prepare` flag

For Docker deployments, provided image includes `ffmpeg`.

#### Thumbnails

Raikiri supports thumbnails for images, videos, and audio files. Thumbnails are stored as hidden files (prefixed with `.`) in the same directory as the media files. When available, thumbnails are displayed in the grid view for quick preview.

To generate thumbnails, use the `-prepare` flag with one of the following modes:
- `videos`: Generate thumbnails recursively for all video files in the current directory
- `video`: Generate thumbnails for video files in the current folder only
- `shows`: Auto-match TV shows using TMDB API (requires `TMDB_API_KEY` environment variable)
- `show`: Manual interactive TV show matching using TMDB API (requires `TMDB_API_KEY` environment variable)

```bash
raikiri -prepare videos
```

Raikiri will intelligently skip files which already have a thumbnail. Video thumbnails are generated at 50% of the video duration. For images, the original file is used as a fallback if no thumbnail exists.

Thumbnails are also supported for the Music mode. Music expects the base directory to have multiple artists, each represented by a directory, containing albums (directories), which in turn contain tracks. If a thumbnail file is present within an album, that becomes the album art; similarly, a thumbnail inside the artist directory becomes the artist cover. Track rows use the list view and do not fetch thumbnails.

#### Player and Playlists

When you click on a media file (image, video, or audio), Raikiri automatically creates a playlist queue from all media files in the current directory. The player bar appears at the bottom of the screen, showing the current item with thumbnail, title, and playback controls. Clicking the player bar expands it to show the full player with seek controls, time display, and a queue dialog button.

The queue dialog displays all items in the current playlist, with the active item highlighted. You can click any item in the queue to jump directly to it. Use the Shuffle button to play all media files recursively from the current directory in random order; non-media files and folders are skipped.

Images automatically advance every 5 seconds when playing. Videos and audio support standard playback controls including play/pause, previous/next, and seeking. Fullscreen mode is available for videos and images.

Raikiri uses HLS (HTTP Live Streaming) for video playback, which provides better compatibility across browsers and formats. Videos are transcoded on-the-fly using `ffmpeg` into HLS segments, enabling smooth playback and seeking even for formats like `.mkv` that may not be directly supported by browsers. Audio files are played directly using HTML5 audio. For files that cannot be played in the browser, they will open in a new tab with a raw GET request.

Raikiri supports subtitles for video playback. External subtitle files in SRT format are automatically detected if they are in the same directory as the video file, or in a `subs/` or `Subs/` subdirectory. Embedded subtitle tracks within video files are also automatically extracted and made available. All subtitles are converted to WebVTT format for web playback. Use the subtitle button (CC icon) in the player to open the subtitle selection dialog, where you can choose from available subtitle tracks or disable subtitles entirely.

#### Quickie on Playback Sync

- Videos are transcoded to HLS format on-the-fly using `ffmpeg`, providing full seekability and compatibility across all video formats.
- Custom fullscreen overlay for video provides dedicated controls (play/pause, +-10s seek, seek bar, exit) while keeping native browser controls hidden.
- Fullscreen button is disabled for audio items (only images/videos use fullscreen).
