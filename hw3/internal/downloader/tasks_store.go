package downloader

import (
	"fmt"
	"sync"
)

type TasksStore struct {
	tasks map[string]*TaskMeta
	lock  sync.Mutex
}

type TaskMeta struct {
	task Task

	completionChan chan struct{}

	sync.Mutex
	//TODO: cancelFunc context.CancelFunc
}

func NewTaskMeta(task Task, completionChan chan struct{}) *TaskMeta {
	return &TaskMeta{task: task, completionChan: completionChan}
}

func (t *TaskMeta) Task() Task {
	return t.task
}

func NewTasksStore() *TasksStore {
	return &TasksStore{tasks: make(map[string]*TaskMeta)}
}

func (s *TasksStore) RemoveTask(taskId string) (*TaskMeta, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	taskMeta, ok := s.tasks[taskId]
	if !ok {
		return nil, fmt.Errorf("Task '%s' not found\n", taskId)
	}

	taskMeta.Lock() // Lock this entry so we know that none other can work with it
	defer taskMeta.Unlock()
	delete(s.tasks, taskId)

	return taskMeta, nil
}

func (s *TasksStore) WithTaskMeta(
	taskId string,
	handler func(*TaskMeta, error),
) {
	s.lock.Lock() // lock whole database from modification

	taskMeta, ok := s.tasks[taskId]
	if !ok {
		handler(
			nil, fmt.Errorf("Task '%s' not found\n", taskId),
		)
		return
	}

	taskMeta.Lock() // lock single entry

	s.lock.Unlock() // unlock database

	handler(taskMeta, nil) // work with entry
	taskMeta.Unlock()      // unlock after entry modification
}

func (s *TasksStore) PushTaskMeta(taskMeta *TaskMeta) {
	// TODO: here should check whether id is not taken
	s.lock.Lock()
	defer s.lock.Unlock()

	s.tasks[taskMeta.task.Id()] = taskMeta
}
