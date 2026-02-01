package protocol

type Message struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Id          string `json:"id"`
	Data        []byte `json:"data,omitempty"`
	ChunkIndex  int    `json:"chunkIndex,omitempty"`
	TotalChunks int    `json:"totalChunks,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
	Allowed     bool   `json:"allowed,omitempty"`
	ChunkSize   int    `json:"chunkSize,omitempty"`
}
