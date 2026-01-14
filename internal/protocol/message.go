package protocol

type Message struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Data        []byte `json:"data,omitempty"`
	ChunkIndex  int    `json:"chunkIndex,omitempty"`
	TotalChunks int    `json:"totalChunks,omitempty"`
}
