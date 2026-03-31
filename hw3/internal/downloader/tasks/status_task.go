package tasks

import (
	. "downloader/internal/downloader"
	"fmt"
)

type StatusTask struct {
	queryTaskId string

	*BaseTask
}

func NewStatusTask(queryTaskId string) *StatusTask {
	return &StatusTask{
		queryTaskId: queryTaskId,
		BaseTask:    NewBaseTask(),
	}
}

func (d *StatusTask) Execute(downloader *Downloader) {
	downloader.EventsChan() <- NewStartedEvent(d.Id())

	downloader.TasksStore().WithTaskMeta(
		d.queryTaskId,
		func(meta *TaskMeta, err error) {
			if err != nil {
				downloader.EventsChan() <- NewErrorEvent(d.Id(), err)
			} else {
				downloadTask, ok := meta.Task().(*DownloadTask)
				if ok {
					progress := downloadTask.progressWriter.Stat()
					downloader.EventsChan() <- NewDataEvent(d.Id(), progress)
				} else {
					downloader.EventsChan() <- NewErrorEvent(
						d.Id(),
						fmt.Errorf(
							"provided task ID corresponds to non-download task",
						),
					)
				}
			}
		},
	)
	downloader.EventsChan() <- NewDoneEvent(d.Id())
}
