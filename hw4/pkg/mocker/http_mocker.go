package mocker

import (
	"errors"
	"strings"
	"sync"
)

// supportedMethods set of supported methods, should be read only.
var supportedMethods = map[string]bool{
	"GET":    true,
	"POST":   true,
	"PUT":    true,
	"DELETE": true,
	"PATCH":  true,
}

// ErrNoConfigurationExists mocker was not yet configured or configuration is empty
var ErrNoConfigurationExists = errors.New("mocker: no configuration exists")

// ErrMethodNotSupported mocker does not support this kind of HTTP method.
// See supportedMethods.
// NOTE: not sure whether such "support" was meant by assignment or when HTTP route
// does not handle such method.
var ErrMethodNotSupported = errors.New("mocker: method not supported")

// ErrSpecificationDiffers occur when RequestSpec Equals return false
var ErrSpecificationDiffers = errors.New("mocker: request specification differs")

// ErrPathNotRegistered occur when no configuration for given URL path
var ErrPathNotRegistered = errors.New("mocker: path not registered")

// ErrMethodNotRegistered occur when no method for existing URL path configuration
var ErrMethodNotRegistered = errors.New("mocker: method not registered")

// HttpMocker handles logic of matching incoming requests to existing configuration.
// Is safe to use given object (all methods) in multiple routines,
// both configuration and request history access is synchronized.
type HttpMocker struct {
	// configEntries stores mocker configuration .
	// First resolve path, then resolve method.
	// Should be accessed under configEntriesMutex R/W lock.
	configEntries      map[string]map[string]*ConfigEntry
	configEntriesMutex sync.RWMutex

	// requestsHistory stores history of incoming requests.
	// Should be accessed under requestsHistoryMutex R/W lock.
	// NOTE: all, requests are stored, not necessarily matched to configuration.
	requestsHistory      []*RequestSpec
	requestsHistoryMutex sync.RWMutex
}

// NewHttpMocker constructs new instance of HttpMocker.
func NewHttpMocker() *HttpMocker {
	return &HttpMocker{
		configEntries:      make(map[string]map[string]*ConfigEntry, 10),
		configEntriesMutex: sync.RWMutex{},

		requestsHistory:      make([]*RequestSpec, 0, 20),
		requestsHistoryMutex: sync.RWMutex{},
	}
}

// SetReply register new route specified by requestSpec, all requests to which
// should be replied with responseSpec.
// When requestSpec method is not supported by mocker, returns ErrMethodNotSupported.
// NOTE: when route specified by requestSpec exists, it will be rewritten,
// creating new ConfigEntry object each time.
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

// Serve returns response specification matching given request specification.
// When error occur, returns all types defined in this file, see above.
// NOTE: provided request spec is stored history.
func (m *HttpMocker) Serve(requestSpec *RequestSpec) (*ResponseSpec, error) {
	//TODO: maybe should not store body or don't store declined requests at all
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

// DumpConfiguration returns copy of running configuration.
// Note: config entries are immutable (as request/response specs are).
func (m *HttpMocker) DumpConfiguration() []*ConfigEntry {
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

// ClearConfiguration remove existing configuration.
func (m *HttpMocker) ClearConfiguration() {
	m.configEntriesMutex.Lock()
	defer m.configEntriesMutex.Unlock()

	m.configEntries = make(map[string]map[string]*ConfigEntry, 0)
}

// ListRequests return copy of requestsHistory field
func (m *HttpMocker) ListRequests() []*RequestSpec {
	m.requestsHistoryMutex.RLock()
	defer m.requestsHistoryMutex.RUnlock()
	return append([]*RequestSpec{}, m.requestsHistory...)
}

// IsMethodSupported verify whether method is within supportedMethods set
// NOTE: method is normalized before verification
func IsMethodSupported(method string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(method))
	return supportedMethods[normalized]
}
