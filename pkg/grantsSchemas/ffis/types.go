package ffis

// FFISMessageDownload is the message payload for FFIS download operations
type FFISMessageDownload struct {
	SourceFileKey string `json:"sourceFileKey"`
	DownloadURL   string `json:"downloadUrl"`
}

// FFISSourceData is the parse FFIS data from a spreadsheet, stored in in  S3
type FFISSourceData struct {
	Bill              string `json:"bill"`
	OpportunityNumber string `json:"opportunityNumber"`
}
