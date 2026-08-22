package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tanq16/raikiri/internal/media"
)

type DeleteNode struct {
	Name     string       `json:"name"`
	Type     string       `json:"type"`
	Children []DeleteNode `json:"children,omitzero"`
}

type deleteRequest struct {
	Mode  string   `json:"mode"`
	Paths []string `json:"paths"`
}

var errInvalidDeletePath = errors.New("invalid delete path")

func (s *Server) resolveDeleteTarget(mode, rel string) (string, error) {
	clean := strings.Trim(filepath.ToSlash(rel), "/")
	if clean == "" || clean == "." {
		return "", errInvalidDeletePath
	}
	full, ok := s.resolveWithinRoot(mode, clean)
	if !ok {
		return "", errInvalidDeletePath
	}
	root, _ := filepath.Abs(filepath.Clean(s.getRoot(mode)))
	if full == root {
		return "", errInvalidDeletePath
	}
	return full, nil
}

// A bare .thumbnail.jpg is folder-wide art shared by every track in it, so it
// only goes when the folder itself does.
func (s *Server) sidecarThumbnail(mode, rel string) string {
	name := path.Base(rel)
	thumbRel := media.GetThumbnailPath(path.Dir(rel), name, media.GetFileType(name, false), mode)
	if filepath.Base(thumbRel) == ".thumbnail.jpg" {
		return ""
	}
	full, ok := s.resolveWithinRoot(mode, thumbRel)
	if !ok {
		return ""
	}
	if _, err := os.Stat(full); err != nil {
		return ""
	}
	return full
}

func (s *Server) deleteTargets(mode, rel string) ([]string, error) {
	full, err := s.resolveDeleteTarget(mode, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return []string{full}, nil
	}
	targets := []string{full}
	if thumb := s.sidecarThumbnail(mode, rel); thumb != "" {
		targets = append(targets, thumb)
	}
	return targets, nil
}

func (s *Server) deleteTree(mode, rel string) (DeleteNode, error) {
	full, err := s.resolveDeleteTarget(mode, rel)
	if err != nil {
		return DeleteNode{}, err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return DeleteNode{}, err
	}
	name := path.Base(strings.Trim(filepath.ToSlash(rel), "/"))
	if info.IsDir() {
		return dirDeleteNode(full, name), nil
	}
	node := DeleteNode{Name: name, Type: deleteNodeType(name)}
	if thumb := s.sidecarThumbnail(mode, rel); thumb != "" {
		node.Children = []DeleteNode{{Name: filepath.Base(thumb), Type: "thumbnail"}}
	}
	return node, nil
}

func dirDeleteNode(full, name string) DeleteNode {
	node := DeleteNode{Name: name, Type: "dir"}
	entries, err := os.ReadDir(full)
	if err != nil {
		return node
	}
	for _, e := range entries {
		if e.IsDir() {
			node.Children = append(node.Children, dirDeleteNode(filepath.Join(full, e.Name()), e.Name()))
			continue
		}
		node.Children = append(node.Children, DeleteNode{Name: e.Name(), Type: deleteNodeType(e.Name())})
	}
	return node
}

func deleteNodeType(name string) string {
	if strings.HasSuffix(name, ".thumbnail.jpg") {
		return "thumbnail"
	}
	return "file"
}

func decodeDeleteRequest(w http.ResponseWriter, r *http.Request) (deleteRequest, bool) {
	var req deleteRequest
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return req, false
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return req, false
	}
	if len(req.Paths) == 0 {
		http.Error(w, "No paths provided", http.StatusBadRequest)
		return req, false
	}
	return req, true
}

func (s *Server) HandleDeletePreview(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeDeleteRequest(w, r)
	if !ok {
		return
	}
	trees := make([]DeleteNode, 0, len(req.Paths))
	for _, rel := range req.Paths {
		node, err := s.deleteTree(req.Mode, rel)
		if err != nil {
			log.Printf("WARN [server] delete preview skipped path=%s: %v", rel, err)
			continue
		}
		trees = append(trees, node)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trees)
}

func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeDeleteRequest(w, r)
	if !ok {
		return
	}
	res := struct {
		Deleted []string `json:"deleted"`
		Errors  []string `json:"errors"`
	}{Deleted: []string{}, Errors: []string{}}

	for _, rel := range req.Paths {
		targets, err := s.deleteTargets(req.Mode, rel)
		if err != nil {
			log.Printf("ERROR [server] delete rejected path=%s: %v", rel, err)
			res.Errors = append(res.Errors, rel)
			continue
		}
		failed := false
		for _, target := range targets {
			if err := os.RemoveAll(target); err != nil {
				log.Printf("ERROR [server] delete failed path=%s: %v", target, err)
				failed = true
			}
		}
		if failed {
			res.Errors = append(res.Errors, rel)
			continue
		}
		log.Printf("INFO [server] deleted mode=%s path=%s", req.Mode, rel)
		res.Deleted = append(res.Deleted, rel)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
