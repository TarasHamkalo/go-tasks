package mocker

// ConfigEntry represents association between request and response specification.
// On modification should be replaced as whole, granting immutability from creation.
type ConfigEntry struct {
	requestSpec  *RequestSpec
	responseSpec *ResponseSpec
}

func NewConfigEntry(
	requestSpec *RequestSpec,
	responseSpec *ResponseSpec,
) *ConfigEntry {
	return &ConfigEntry{requestSpec: requestSpec, responseSpec: responseSpec}
}

// Matches tests whether provided request match to configured one
func (c *ConfigEntry) Matches(requestSpec *RequestSpec) bool {
	return c.requestSpec.Equals(requestSpec)
}

func (c *ConfigEntry) RequestSpec() *RequestSpec {
	return c.requestSpec
}

func (c *ConfigEntry) ResponseSpec() *ResponseSpec {
	return c.responseSpec
}
