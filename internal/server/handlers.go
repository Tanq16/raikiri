package server

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tanq16/raikiri/internal/media"
)

// HandleContent serves raw files from the media/music directory.
func (s *Server) HandleContent(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	relPath := strings.TrimPrefix(r.URL.Path, "/content/")
	fullPath := filepath.Join(s.getRoot(mode), relPath)
	http.ServeFile(w, r, fullPath)
}

// HandleList returns a JSON array of FileEntry for a directory.
func (s *Server) HandleList(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	relPath := r.URL.Query().Get("path")
	recursive := r.URL.Query().Get("recursive") == "true"

	root := s.getRoot(mode)
	targetDir := filepath.Join(root, relPath)

	var entries []media.FileEntry

	if recursive {
		err := filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if rel == "." {
				return nil
			}

			if d.IsDir() {
				entries = append(entries, media.FileEntry{
					Name:     name,
					Path:     rel,
					Type:     "folder",
					Size:     "",
					Thumb:    media.GetThumbnailPath(rel, name, "folder", mode),
					Modified: media.FormatModTime(info.ModTime()),
				})
				return nil
			}

			fType := media.GetFileType(name, false)
			if fType == "audio" || fType == "video" || fType == "image" {
				dir := filepath.Dir(rel)
				entries = append(entries, media.FileEntry{
					Name:     name,
					Path:     rel,
					Type:     fType,
					Size:     media.FormatFileSize(info.Size()),
					Thumb:    media.GetThumbnailPath(dir, name, fType, mode),
					Modified: media.FormatModTime(info.ModTime()),
				})
			}
			return nil
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	} else {
		files, err := os.ReadDir(targetDir)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		for _, f := range files {
			if strings.HasPrefix(f.Name(), ".") {
				continue
			}

			info, err := f.Info()
			if err != nil {
				continue
			}
			size := ""
			if !f.IsDir() {
				size = media.FormatFileSize(info.Size())
			}

			fType := media.GetFileType(f.Name(), f.IsDir())

			fullRelPath := filepath.Join(relPath, f.Name())
			fullRelPath = filepath.ToSlash(fullRelPath)
			thumbBasePath := relPath
			if fType == "folder" {
				thumbBasePath = fullRelPath
			}

			entries = append(entries, media.FileEntry{
				Name:     f.Name(),
				Path:     fullRelPath,
				Type:     fType,
				Size:     size,
				Thumb:    media.GetThumbnailPath(thumbBasePath, f.Name(), fType, mode),
				Modified: media.FormatModTime(info.ModTime()),
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type == "folder" && entries[j].Type != "folder" {
			return true
		}
		if entries[i].Type != "folder" && entries[j].Type == "folder" {
			return false
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// HandleUpload accepts multipart file uploads.
func (s *Server) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mode := r.FormValue("mode")
	relPath := r.FormValue("path")

	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	files := r.MultipartForm.File["files"]
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		dstPath := filepath.Join(s.getRoot(mode), relPath, fileHeader.Filename)
		dst, err := os.Create(dstPath)
		if err != nil {
			file.Close()
			http.Error(w, err.Error(), 500)
			return
		}

		if _, err := io.Copy(dst, file); err != nil {
			file.Close()
			dst.Close()
			http.Error(w, err.Error(), 500)
			return
		}

		file.Close()
		dst.Close()
	}

	w.WriteHeader(200)
}
