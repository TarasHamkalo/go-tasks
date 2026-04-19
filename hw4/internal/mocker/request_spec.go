package mocker

import (
	"bytes"
	"fmt"
	"strings"
)

type RequestSpec struct {
	path string
	// contains pairs "key:value:freq" as keys for later comparison
	queryParams map[string]any
	method      string
	body        []byte
}

func NewRequestSpec(
	path string,
	method string,
	rawQueryParams map[string][]string,
	body []byte,
) *RequestSpec {
	queryParams := map[string]any{}
	for key, values := range rawQueryParams {
		valueFrequencies := make(map[string]int, len(values))
		for _, value := range values {
			freq, ok := valueFrequencies[value]
			if !ok {
				freq = 0
			}

			valueFrequencies[value] = freq + 1
		}
		for value, freq := range valueFrequencies {
			s := fmt.Sprintf("%s:%s:%d", key, value, freq)
			queryParams[s] = 1
		}
	}

	return &RequestSpec{
		path:        path,
		queryParams: queryParams,
		method:      strings.ToUpper(method),
		body:        body,
	}
}

func (r *RequestSpec) Equals(other *RequestSpec) bool {
	if r.method != other.method || r.path != other.path {
		return false
	}

	if len(other.queryParams) != len(r.queryParams) {
		return false
	}

	for param := range r.queryParams {
		if _, present := other.queryParams[param]; !present {
			return false
		}
	}

	return bytes.Equal(r.body, other.body)
}

func (r *RequestSpec) Method() string {
	return r.method
}

func (r *RequestSpec) Path() string {
	return r.path
}

type RequestSpecBuilder struct {
	path           string
	rawQueryParams map[string][]string
	method         string
	body           []byte
}

func NewRequestSpecBuilder(path string, method string) *RequestSpecBuilder {
	return &RequestSpecBuilder{
		path:           path,
		method:         method,
		body:           make([]byte, 0),
		rawQueryParams: map[string][]string{},
	}
}

func (b *RequestSpecBuilder) WithQueryParams(
	rawQueryParams map[string][]string,
) *RequestSpecBuilder {
	b.rawQueryParams = rawQueryParams
	return b
}

func (b *RequestSpecBuilder) WithBody(body []byte) *RequestSpecBuilder {
	b.body = body
	return b
}

// TODO: consider allocating copies
func (b *RequestSpecBuilder) Build() *RequestSpec {
	return NewRequestSpec(
		b.path,
		b.method,
		b.rawQueryParams,
		b.body,
	)
}
