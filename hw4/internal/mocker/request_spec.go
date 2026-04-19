package mocker

import (
	"bytes"
	"fmt"
	"strings"
)

type RequestSpec struct {
	path string
	// contains pairs "key:value:freq" as keys for later comparison
	queryParams map[string]struct{}
	method      string

	hasBody bool
	body    []byte

	rawQueryParams map[string][]string
}

func NewRequestSpec(
	path string,
	method string,
	rawQueryParams map[string][]string,
	hasBody bool,
	body []byte,
) *RequestSpec {
	queryParams := map[string]struct{}{}
	rawQueryParamsCopy := map[string][]string{}
	for key, values := range rawQueryParams {
		rawQueryParamsCopy[key] = append([]string{}, values...)
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
			queryParams[s] = struct{}{}
		}
	}

	normalized := strings.ToUpper(strings.TrimSpace(method))
	return &RequestSpec{
		path:           path,
		queryParams:    queryParams,
		method:         normalized,
		hasBody:        hasBody,
		body:           body,
		rawQueryParams: rawQueryParamsCopy,
	}
}

func (r *RequestSpec) Equals(other *RequestSpec) bool {
	if r.method != other.method || r.path != other.path {
		return false
	}

	if len(other.queryParams) != len(r.queryParams) {
		return false
	}

	if other.hasBody != r.hasBody {
		return false
	}

	for param := range r.queryParams {
		if _, present := other.queryParams[param]; !present {
			return false
		}
	}

	if r.hasBody && !bytes.Equal(r.body, other.body) {
		return false
	}

	return true
}

func (r *RequestSpec) QueryParams() map[string][]string {
	return r.rawQueryParams
}

func (r *RequestSpec) HasBody() bool {
	return r.hasBody
}

func (r *RequestSpec) Body() []byte {
	return append([]byte{}, r.body...)
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
	hasBody        bool
	body           []byte
}

func NewRequestSpecBuilder(path string, method string) *RequestSpecBuilder {
	return &RequestSpecBuilder{
		path:           path,
		method:         method,
		hasBody:        false,
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
	b.hasBody = true
	b.body = body
	return b
}

// TODO: consider allocating copies
func (b *RequestSpecBuilder) Build() *RequestSpec {
	return NewRequestSpec(
		b.path,
		b.method,
		b.rawQueryParams,
		b.hasBody,
		b.body,
	)
}
