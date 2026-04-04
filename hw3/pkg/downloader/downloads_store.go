package downloader

import (
	"fmt"
	"sync"
)

// DownloadsStore is thread safe table like storage for DownloadRecord.
type DownloadsStore struct {
	downloads map[string]*DownloadRecord
	lock      sync.RWMutex
}

func NewDownloadsStore() *DownloadsStore {
	return &DownloadsStore{
		downloads: make(map[string]*DownloadRecord, 10),
		lock:      sync.RWMutex{},
	}
}

func (s *DownloadsStore) Add(d *DownloadRecord) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	_, ok := s.downloads[d.Id()]
	if ok {
		return fmt.Errorf("download with id %s already stored", d.Id())
	}

	s.downloads[d.Id()] = d

	return nil
}

func (s *DownloadsStore) Get(id string) (*DownloadRecord, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	d, ok := s.downloads[id]
	if ok {
		return d, nil
	}

	return nil, fmt.Errorf("download %s not found", id)
}

// TODO: maybe cache :) ?
func (s *DownloadsStore) GetAllViews() []*DownloadView {
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

func (s *DownloadsStore) GetAllIds() []string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	result := make([]string, 0, len(s.downloads))
	for _, d := range s.downloads {
		result = append(result, d.Id())
	}

	return result
}
