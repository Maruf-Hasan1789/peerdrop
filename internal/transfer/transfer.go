package transfer

type Status int

const (
	Pending Status = iota
	InProgress
	Paused
	Completed
	Failed
)

type Transfer struct {
	FileId    string
	PeerId    string
	Status    Status
	ChunkDone int64
}
