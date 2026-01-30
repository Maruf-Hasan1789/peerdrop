package domain

type FileMetadata struct {
	ID       string `json:"id"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	Checksum string `json:"checksum"`
}
