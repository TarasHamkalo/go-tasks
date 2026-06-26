package downloader

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// TaskEntry associates task with its context cancellation function.
type TaskEntry struct {
	Task   *DownloadTask
	Cancel context.CancelFunc
}

func NewTaskEntry(task *DownloadTask, cancel context.CancelFunc) *TaskEntry {
	return &TaskEntry{Task: task, Cancel: cancel}
}

// TasksStore is thread safe table like storage for DownloadTask.
// Allows transition to "Drain only" mode, when no tasks are added and only
// removal can be triggered (used for shutdown process in Downloader).
type TasksStore struct {
	entries map[string]*TaskEntry

	// drainOnly indicates whether in drain only mode
	drainOnly atomic.Bool

	// drainedChan is closed after drain mode was entered and all tasks removed
	drainedChan chan struct{}

	// drainOnce wrapper to close drainedChan only once
	drainOnce sync.Once

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
// no tasks left.
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

// CancelAll is helper method to cancel all tasks under write lock.
func (s *TasksStore) CancelAll() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, entry := range s.entries {
		entry.Cancel()
	}
}

// Remove under write locks removes task with given id, in case of drain mode,
// closes drainedChan.
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

// Get find task by id under read lock.
func (s *TasksStore) Get(id string) (*TaskEntry, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	entry, ok := s.entries[id]
	if ok {
		return entry, nil
	}

	return nil, fmt.Errorf("task %s not found", id)
}

// Add adds task entry under write lock in case drain mode is not active.
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
