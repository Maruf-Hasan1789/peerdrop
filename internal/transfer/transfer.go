package transfer

import "context"

type Status int
type Direction int

const (
	Incoming Direction = iota
	Outgoing
)

const (
	Pending Status = iota
	Rejected
	InProgress
	Paused
	Completed
	Failed
)

type Transfer struct {
	PeerId           string
	TransferId       string
	RootID           string
	RootName         string
	Status           Status
	TransferredBytes int64
	TotalBytes       int64
	Direction        Direction
}

type TransferTask struct {
	Meta   *Transfer
	Ctx    context.Context
	Cancel context.CancelFunc
}
