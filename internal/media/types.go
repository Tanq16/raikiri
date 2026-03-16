package media

type FileEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Size     string `json:"size"`
	Thumb    string `json:"thumb,omitempty"`
	Modified string `json:"modified,omitempty"`
}

type AudioTrack struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Channels int    `json:"channels"`
}

type SubtitleTrack struct {
	Index int    `json:"index"`
	Codec string `json:"codec"`
}
