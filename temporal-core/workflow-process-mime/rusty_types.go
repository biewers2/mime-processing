package workflow_process_mime

type CreateRustyWorkspaceInput struct{}

type CreateRustyWorkspaceOutput struct {
	SourcePath string `json:"source_path"`
	OutputDir  string `json:"output_dir"`
}

type DestroyRustyWorkspaceInput struct {
	SourcePath string `json:"source_path"`
	OutputDir  string `json:"output_dir"`
}

type DestroyRustyWorkspaceOutput struct{}

type DownloadRustyFileInput struct {
	SourceS3Uri string `json:"source_s3_uri"`
	OutputPath  string `json:"output_file_path"`
}

type DownloadRustyFileOutput struct {
	Bytes int64 `json:"bytes"`
}

type ProcessRustyFileInput struct {
	SourcePath string `json:"source_path"`
	OutputDir  string `json:"output_dir"`
	Mimetype   string `json:"mimetype"`
}

type ProcessRustyFileOutput struct {
	Processed []FileInfo    `json:"processed"`
	Embedded  []FileInfo    `json:"embedded"`
	Failures  []FailureInfo `json:"failures"`
}

type FileInfo struct {
	Path     string `json:"path"`
	MimeType string `json:"mimetype"`
	DupeId   string `json:"dupe_id"`
}

type EmbeddedS3Info struct {
	EmbeddedS3Uri string `json:"embedded_s3_uri"`
	MimeType      string `json:"mimetype"`
	DupeId        string `json:"dupe_id"`
}

type FailureInfo struct {
	Message string `json:"message"`
}

type UploadRustyFileInput struct {
	SourceFilePath string `json:"source_file_path"`
	MimeType       string `json:"mimetype"`
	OutputS3Uri    string `json:"output_s3_uri"`
}
