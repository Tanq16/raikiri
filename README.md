<div align="center">
  <img src=".github/assets/logo.png" alt="Local Content Share Logo" width="200">
  <h1>Raikiri</h1>

  <a href="https://github.com/tanq16/raikiri/actions/workflows/release.yml"><img alt="Build Workflow" src="https://github.com/tanq16/raikiri/actions/workflows/release.yml/badge.svg"></a>&nbsp;<a href="https://github.com/Tanq16/raikiri/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/tanq16/raikiri"></a>&nbsp;<a href="https://hub.docker.com/r/tanq16/raikiri"><img alt="Docker Pulls" src="https://img.shields.io/docker/pulls/tanq16/raikiri"></a><br><br>
</div>

---

A fast, simple, self-hosted, no-nonsense app for running a media server. This is meant for those instances when you don't need something beefy like Jellyfin or Plex, and don't want to go through the pain of metadata tagging.

The aim of the application is to provide directory listing in an elegant interface to view images and videos easily. There is no need for metadata and match, Raikiri just uses the folder navigation to display things. While Raikiri only displays common image and video formats, other files will also be displayed to download directly.

Ideally, you can just run the service and it will be fine. But large images can easily slow the frontend down when fetching an entire directory of them. For such instances, Raikiri provides a way to generate thumbnails for video and image files using `ffmpeg`. You can manually run this per directory or on your entire directory periodically to ensure thumbnails are up to date. Raikiri will intelligently select thumbnails and display as needed.

## Screenshots

| Desktop View | Mobile View |
| --- | --- |
| <img src=".github/assets/df.png" alt="Light"> | <img src=".github/assets/mf.png" alt="Light"> |
| <img src=".github/assets/di.png" alt="Light"> | <img src=".github/assets/mi.png" alt="Light"> |

## Usage

### Docker (for Homelab)

```bash
mkdir $HOME/raikiri # you don't need to create this if you already have media in a specific directory
```
```bash
docker run --rm -d --name raikiri \
  -p 8080:8080 \
  -v $HOME/raikiri:/app/media \
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
    ports:
      - 8080:8080
```

### Binary

To use the binary, simply download the latest version from the project releases and run as follows:

```bash
raikiri -media $YOUR_MEDIA_FOLDER
```

To prepare the thumbnails, use:

```bash
raikiri -prepare -media $YOUR_MEDIA_FOLDER
```

Additionally, you can use also use the optional parameters of `-port` to specify a port of your choice, and `-refresh` to specify number of minutes in which to refresh the list of files.

### Local development

With `Go 1.23+` installed, run the following to download the binary to your GOBIN:

```bash
go install github.com/tanq16/raikiri@latest
```

Or, you can build from source like so:

```bash
git clone https://github.com/tanq16/raikiri.git && \
cd raikiri && \
go build .
```
