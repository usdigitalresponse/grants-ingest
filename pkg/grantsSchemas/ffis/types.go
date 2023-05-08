package ffis

// message payload for FFIS download operations
type FFISMessageDownload struct {
	SourceFileKey string `json:"sourceFileKey"`
	DownloadURL   string `json:"downloadUrl"`
}
