package transfer

type Status int
type Direction int

const (
	Incoming Direction = iota
	Outgoing
)

const (
	Pending Status = iota
	InProgress
	Paused
	Completed
	Failed
)

type Transfer struct {
	PeerId        string
	FileId        string
	FileName      string
	Status        Status
	ChunkReceived int64
	TotalChunks   int64
	Direction     Direction
}
