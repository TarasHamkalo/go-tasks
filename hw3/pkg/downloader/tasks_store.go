package downloader

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

type TaskEntry struct {
	Task   *DownloadTask
	Cancel context.CancelFunc
}

func NewTaskEntry(task *DownloadTask, cancel context.CancelFunc) *TaskEntry {
	return &TaskEntry{Task: task, Cancel: cancel}
}

type TasksStore struct {
	entries map[string]*TaskEntry

	drainOnly atomic.Bool

	drainedChan chan struct{}
	drainOnce   sync.Once

	lock sync.RWMutex
}

func NewTasksStore() *TasksStore {

	return &TasksStore{
		entries: make(map[string]*TaskEntry, 10),

		drainOnly: atomic.Bool{},

		drainedChan: make(chan struct{}),

		lock: sync.RWMutex{},
	}
}

// DrainOnly sets tasks store in mode, during which tasks can only be
// read or removed. DrainOnly returns chan which gonna be closed when
// no tasks left
func (s *TasksStore) DrainOnly() <-chan struct{} {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.drainOnly.Store(true)
	if len(s.entries) == 0 {
		s.drainOnce.Do(func() {
			close(s.drainedChan)
		})
	}

	return s.drainedChan
}

func (s *TasksStore) CancelAll() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, entry := range s.entries {
		entry.Cancel()
	}
}

func (s *TasksStore) Remove(taskId string) (*TaskEntry, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	entry, ok := s.entries[taskId]
	if ok {
		delete(s.entries, taskId)
		if s.drainOnly.Load() && len(s.entries) == 0 {
			s.drainOnce.Do(func() {
				close(s.drainedChan)
			})
		}

		return entry, nil
	}

	return nil, fmt.Errorf("task %s not found", taskId)
}

func (s *TasksStore) Get(id string) (*TaskEntry, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	entry, ok := s.entries[id]
	if ok {
		return entry, nil
	}

	return nil, fmt.Errorf("task %s not found", id)
}

func (s *TasksStore) Add(entry *TaskEntry) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.drainOnly.Load() {
		return fmt.Errorf("tasks store in drain only mode, no new tasks accepted")
	}

	_, ok := s.entries[entry.Task.Id()]
	if ok {
		return fmt.Errorf("task with id %s already stored", entry.Task.Id())
	}

	s.entries[entry.Task.Id()] = entry
	return nil
}
