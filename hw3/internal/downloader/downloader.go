package downloader

import "fmt"

type Downloader struct {
	userAgent string

	eventsChan chan Event

	tasksStore *TasksStore
}

func (d *Downloader) TasksStore() *TasksStore {
	return d.tasksStore
}

func NewDownloader() *Downloader {
	return &Downloader{
		userAgent:  "BOT FIT/CTU (student project)",
		eventsChan: make(chan Event), // TODO: capacity?
		tasksStore: NewTasksStore(),
	}
}

func (d *Downloader) Start() {
	go (func() {
		for event := range d.eventsChan {
			fmt.Printf("[%s] [%s]\n", event.TaskId(), event.EventType())
			fmt.Println(event.Properties())

			if event.EventType() == TaskDone {
				d.tasksStore.WithTaskMeta(
					event.TaskId(),
					func(taskMeta *TaskMeta, err error) {
						if err == nil {
							taskMeta.completionChan <- struct{}{}
						}
					},
				)
			}
		}
	})()
}

func (d *Downloader) Submit(task Task) <-chan struct{} {
	completionChan := make(chan struct{}, 1)

	d.tasksStore.PushTaskMeta(NewTaskMeta(task, completionChan))
	go task.Execute(d)

	return completionChan
}

func (d *Downloader) UserAgent() string {
	return d.userAgent
}

func (d *Downloader) EventsChan() chan<- Event {
	return d.eventsChan
}
