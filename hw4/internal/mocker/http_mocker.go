package mocker

import (
	"errors"
	"strings"
	"sync"
)

var supportedMethods = map[string]bool{
	"GET":  true,
	"POST": true,
}

var ErrNoConfigurationExists = errors.New("mocker: no configuration exists")
var ErrMethodNotSupported = errors.New("mocker: method not supported")
var ErrSpecificationDiffers = errors.New("mocker: request specification differs")
var ErrPathNotRegistered = errors.New("mocker: path not registered")
var ErrMethodNotRegistered = errors.New("mocker: method not registered")

type HttpMocker struct {
	// resolve path, then resolve method
	configEntries      map[string]map[string]*ConfigEntry
	configEntriesMutex sync.RWMutex

	// stores history of requests (all, not necessarily matched)
	requestsHistory      []*RequestSpec
	requestsHistoryMutex sync.RWMutex
}

func NewHttpMocker() *HttpMocker {
	return &HttpMocker{
		configEntries:      make(map[string]map[string]*ConfigEntry, 10),
		configEntriesMutex: sync.RWMutex{},
	}
}

func (m *HttpMocker) SetReply(
	requestSpec *RequestSpec,
	responseSpec *ResponseSpec,
) error {
	if !IsMethodSupported(requestSpec.Method()) {
		return ErrMethodNotSupported
	}

	m.configEntriesMutex.Lock()
	defer m.configEntriesMutex.Unlock()

	methodsMap, ok := m.configEntries[requestSpec.Path()]
	if !ok {
		methodsMap = make(map[string]*ConfigEntry, 1)
		m.configEntries[requestSpec.Path()] = methodsMap
	}
	methodsMap[requestSpec.Method()] = NewConfigEntry(requestSpec, responseSpec)
	return nil
}

func (m *HttpMocker) Serve(requestSpec *RequestSpec) (*ResponseSpec, error) {
	m.requestsHistoryMutex.Lock()
	m.requestsHistory = append(m.requestsHistory, requestSpec)
	m.requestsHistoryMutex.Unlock()

	if !IsMethodSupported(requestSpec.Method()) {
		return nil, ErrMethodNotSupported
	}

	m.configEntriesMutex.RLock()
	defer m.configEntriesMutex.RUnlock()
	if len(m.configEntries) == 0 {
		return nil, ErrNoConfigurationExists
	}

	methodsMap, ok := m.configEntries[requestSpec.Path()]
	if !ok {
		return nil, ErrPathNotRegistered
	}

	configEntry, ok := methodsMap[requestSpec.Method()]
	if !ok {
		return nil, ErrMethodNotRegistered
	}

	if configEntry.Matches(requestSpec) {
		return configEntry.ResponseSpec(), nil
	}

	return nil, ErrSpecificationDiffers
}

func (m *HttpMocker) DumpConfiguration() []*ConfigEntry {
	// note: config entries are immutable (as request/response specs are)
	m.configEntriesMutex.RLock()
	defer m.configEntriesMutex.RUnlock()

	entries := make([]*ConfigEntry, 0, len(m.configEntries))
	for _, methodMap := range m.configEntries {
		for _, entry := range methodMap {
			entries = append(entries, entry)
		}
	}

	return entries
}

func (m *HttpMocker) ClearConfiguration() {
	m.configEntriesMutex.Lock()
	defer m.configEntriesMutex.Unlock()

	m.configEntries = make(map[string]map[string]*ConfigEntry, 0)
}

func (m *HttpMocker) ListRequests() ([]*RequestSpec, error) {
	m.requestsHistoryMutex.RLock()
	defer m.requestsHistoryMutex.RUnlock()
	return append([]*RequestSpec{}, m.requestsHistory...), nil
}

func IsMethodSupported(method string) bool {
	return supportedMethods[strings.ToUpper(method)]
}
