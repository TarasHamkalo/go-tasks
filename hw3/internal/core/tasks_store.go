package core

import (
	"fmt"
	"sync"
)

type TasksStore struct {
	tasks map[string]*DownloadTask
	lock  sync.RWMutex
}

func NewTasksStore() *TasksStore {
	return &TasksStore{
		tasks: make(map[string]*DownloadTask, 10),
		lock:  sync.RWMutex{},
	}
}

func (s *TasksStore) Remove(taskId string) (*DownloadTask, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	task, ok := s.tasks[taskId]
	if ok {
		delete(s.tasks, taskId)
		return task, nil
	}

	return nil, fmt.Errorf("task %s not found", taskId)
}

func (s *TasksStore) Get(id string) (*DownloadTask, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	d, ok := s.tasks[id]
	if ok {
		return d, nil
	}

	return nil, fmt.Errorf("task %s not found", id)
}

func (s *TasksStore) Add(task *DownloadTask) {
	// TODO: here should check whether id is not taken
	s.lock.Lock()
	defer s.lock.Unlock()

	s.tasks[task.Id()] = task
}
