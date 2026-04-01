package core

import (
	"fmt"
	"sync"
)

type DownloadStore struct {
	downloads map[string]*DownloadRecord
	lock      sync.RWMutex
}

func NewDownloadStore() *DownloadStore {
	return &DownloadStore{
		downloads: make(map[string]*DownloadRecord, 10),
		lock:      sync.RWMutex{},
	}
}

func (s *DownloadStore) Add(d *DownloadRecord) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.downloads[d.id] = d
}

func (s *DownloadStore) Get(id string) (*DownloadRecord, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	d, ok := s.downloads[id]
	if ok {
		return d, nil
	}

	return nil, fmt.Errorf("download %s not found", id)
}

// TODO: maybe cache :) ?
func (s *DownloadStore) GetAllViews() []*DownloadView {
	s.lock.RLock()
	records := make([]*DownloadRecord, 0, len(s.downloads))

	// get pointers to downloads
	for _, d := range s.downloads {
		records = append(records, d)
	}
	s.lock.RUnlock()

	// make copy outside map lock
	result := make([]*DownloadView, 0, len(records))
	for _, d := range records {
		result = append(result, d.DetachedView())
	}

	return result
}
