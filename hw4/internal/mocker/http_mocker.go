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
	servedRequests []*RequestSpec
}

func NewHttpMocker() *HttpMocker {
	return &HttpMocker{
		configEntries:      make(map[string]map[string]*ConfigEntry, 10),
		configEntriesMutex: sync.RWMutex{},
	}
}

func (m *HttpMocker) SetRoute(requestSpec *RequestSpec, responseSpec *ResponseSpec) {
	m.configEntriesMutex.Lock()
	defer m.configEntriesMutex.Unlock()

	methodsMap, ok := m.configEntries[requestSpec.Path()]
	if !ok {
		methodsMap = make(map[string]*ConfigEntry, 1)
		m.configEntries[requestSpec.Path()] = methodsMap
	}

	entry, ok := methodsMap[requestSpec.Method()]
	if ok {
		entry.SetResponseSpec(responseSpec)
	} else {
		methodsMap[requestSpec.Method()] = NewConfigEntry(requestSpec, responseSpec)
	}
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

	if configEntry.RequestSpec().Equals(requestSpec) {
		// should append only from configuration entry so that no new allocations done
		m.servedRequests = append(m.servedRequests, configEntry.requestSpec)
		return configEntry.ResponseSpec(), nil
	}

	return nil, ErrSpecificationDiffers
}

func IsMethodSupported(method string) bool {
	return supportedMethods[strings.ToUpper(method)]
}
