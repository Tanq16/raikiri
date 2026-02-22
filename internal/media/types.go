package media

// FileEntry represents a file or directory in a listing response.
type FileEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Size     string `json:"size"`
	Thumb    string `json:"thumb,omitempty"`
	Modified string `json:"modified,omitempty"`
}

// AudioTrack holds metadata for one audio stream in a container.
type AudioTrack struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Channels int    `json:"channels"`
}

// SubtitleTrack holds metadata for one subtitle stream.
type SubtitleTrack struct {
	Index int    `json:"index"`
	Codec string `json:"codec"`
}
