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

	// stores history of served requests
	// TODO: locking
	servedRequests []*RequestSpec
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

	entry, ok := methodsMap[requestSpec.Method()]
	if ok {
		entry.Lock()
		defer entry.Unlock()

		entry.SetResponseSpec(responseSpec)
	} else {
		methodsMap[requestSpec.Method()] = NewConfigEntry(requestSpec, responseSpec)
	}

	return nil
}

func (m *HttpMocker) Serve(requestSpec *RequestSpec) (*ResponseSpec, error) {
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

	configEntry.RLock()
	defer configEntry.RUnlock()

	if configEntry.RequestSpec().Equals(requestSpec) {
		// should append only from configuration entry so that no new allocations done
		m.servedRequests = append(m.servedRequests, configEntry.requestSpec)
		return configEntry.ResponseSpec(), nil
	}

	return nil, ErrSpecificationDiffers
}

func (m *HttpMocker) DumpConfiguration() []*ConfigEntry {
	// note: pointers to Response/Request specifications are copied
	// at the same time those objects are immutable, so internal state not exposed to mutation
	// by this call
	entries := make([]*ConfigEntry, len(m.configEntries))
	for _, methodMap := range m.configEntries {
		for _, entry := range methodMap {
			entry.RLock()
			entries = append(entries, NewConfigEntry(
				entry.requestSpec,
				entry.responseSpec,
			))
			entry.RUnlock()
		}
	}
	return entries
}

func (m *HttpMocker) ClearConfiguration() {
	m.configEntriesMutex.Lock()
	defer m.configEntriesMutex.Unlock()

	m.configEntries = make(map[string]map[string]*ConfigEntry, 10)
}

func (m *HttpMocker) ListRequests() ([]*RequestSpec, error) {
	m.configEntriesMutex.RLock()
	defer m.configEntriesMutex.RUnlock()
	return m.servedRequests, nil
}

func IsMethodSupported(method string) bool {
	return supportedMethods[strings.ToUpper(method)]
}
