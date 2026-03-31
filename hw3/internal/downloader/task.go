package downloader

import (
	"github.com/google/uuid"
)

// Task represents task executed by downloader.
// Errors gonna be written to errors channel of downloader.
type Task interface {
	Id() string

	Execute(*Downloader)
}

type BaseTask struct {
	id string
}

func NewBaseTask() *BaseTask {
	return &BaseTask{
		id: uuid.New().String(),
	}
}

func (b *BaseTask) Id() string {
	return b.id
}

func (b *BaseTask) Execute(downloader *Downloader) {}
