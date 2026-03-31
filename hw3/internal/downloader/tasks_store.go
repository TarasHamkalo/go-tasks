package downloader

import (
	"fmt"
	"sync"
)

type TasksStore struct {
	tasks map[string]*DownloadTask
	lock  sync.Mutex
}

func NewTasksStore() *TasksStore {
	return &TasksStore{
		tasks: make(map[string]*DownloadTask, 10),
		lock:  sync.Mutex{},
	}
}

func (s *TasksStore) RemoveTask(taskId string) (*DownloadTask, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	task, ok := s.tasks[taskId]
	if ok {
		delete(s.tasks, taskId)
		return task, nil
	}

	return nil, fmt.Errorf("task %s not found", taskId)
}

func (s *TasksStore) AddTask(task *DownloadTask) {
	// TODO: here should check whether id is not taken
	s.lock.Lock()
	defer s.lock.Unlock()

	s.tasks[task.Id()] = task
}
