package protocol

const ProtocolVersion int = 1

type MessageType string

const (
	TypeHandshake MessageType = "HANDSHAKE"
	TypeChunk     MessageType = "CHUNK"
	TypeAck       MessageType = "ACK"
	TypeControl   MessageType = "CONTROL"
	TypeDone      MessageType = "DONE"
)

type ControlAction string

const (
	ActionHandshakeAck ControlAction = "HANDSHAKE_ACK"
	ActionPause        ControlAction = "PAUSE"
	ActionResume       ControlAction = "RESUME"
	ActionError        ControlAction = "ERROR"
)

type RootEntryType string

const (
	EntryTypeFile      RootEntryType = "FILE"
	EntryTypeDirectory RootEntryType = "DIRECTORY"
)

type Message struct {
	Version   int         `json:"version"`
	Type      MessageType `json:"type"`
	ID        string      `json:"id"`         //transferId
	SessionID string      `json:"session_id"` //sessionId

	Handshake *Handshake `json:"handshake,omitempty"`
	Chunk     *Chunk     `json:"chunk,omitempty"`
	Ack       *Ack       `json:"ack,omitempty"`
	Resume    *Resume    `json:"resume,omitempty"`
	Control   *Control   `json:"control,omitempty"`
	Done      bool       `json:"done"`
}

type Handshake struct {
	Roots         []*RootEntry `json:"roots"`
	TotalSize     int64        `json:"total_size"`
	ChunkSize     int          `json:"chunk_size"`
	ParallelChunk int          `json:"parallel_chunk"`
}

type RootEntry struct {
	ID    string        `json:"id"`
	Name  string        `json:"name"`
	Type  RootEntryType `json:"type"`
	Files []FileMeta    `json:"files"`
	Size  int64         `json:"size"`
}

type FileMeta struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	CheckSum string `json:"checksum"`
}

type Chunk struct {
	RootId   string `json:"root_id"`
	FileID   string `json:"file_id"`
	Index    int    `json:"index"`
	Total    int    `json:"total"`
	Offset   int64  `json:"offset"`
	Size     int64  `json:"size"`
	Data     []byte `json:"data"`
	CheckSum string `json:"checksum"`
}

type Ack struct {
	FileID string `json:"file_id"`
	Index  int    `json:"index"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

type Resume struct {
	FileID  string `json:"file_id"`
	Missing []int  `json:"missing"`
}

type Control struct {
	Action ControlAction `json:"action,omitempty"`

	Files    []FileControl `json:"files,omitempty"`
	AllowAll bool          `json:"allow_all,omitempty"`
	Details  string        `json:"details,omitempty"`
}

type FileControl struct {
	FileID  string `json:"file_id"`
	Allowed bool   `json:"allowed"`
}

type Done struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
