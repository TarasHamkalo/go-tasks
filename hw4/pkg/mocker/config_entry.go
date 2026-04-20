package mocker

type ConfigEntry struct {
	requestSpec  *RequestSpec
	responseSpec *ResponseSpec
}

func NewConfigEntry(requestSpec *RequestSpec, responseSpec *ResponseSpec) *ConfigEntry {
	return &ConfigEntry{requestSpec: requestSpec, responseSpec: responseSpec}
}

func (c *ConfigEntry) Matches(requestSpec *RequestSpec) bool {
	return c.requestSpec.Equals(requestSpec)
}

func (c *ConfigEntry) RequestSpec() *RequestSpec {
	return c.requestSpec
}

func (c *ConfigEntry) ResponseSpec() *ResponseSpec {
	return c.responseSpec
}
