package mocker

import "sync"

type ConfigEntry struct {
	// immutable from creation of config entry
	requestSpec *RequestSpec

	// swappable as whole
	responseSpec *ResponseSpec

	sync.RWMutex
}

func NewConfigEntry(requestSpec *RequestSpec, responseSpec *ResponseSpec) *ConfigEntry {
	return &ConfigEntry{requestSpec: requestSpec, responseSpec: responseSpec}
}
func (c *ConfigEntry) SetResponseSpec(spec *ResponseSpec) {
	c.responseSpec = spec
}

func (c *ConfigEntry) RequestSpec() *RequestSpec {
	return c.requestSpec
}

func (c *ConfigEntry) ResponseSpec() *ResponseSpec {
	return c.responseSpec
}
