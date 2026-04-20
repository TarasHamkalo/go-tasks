package mocker

import (
	"bytes"
	"strings"
)

type RequestSpec struct {
	path string
	// key to value:frequency pairs
	// initially used set of "key:value:freq" elements, but hard to parse back
	query map[string]map[string]int

	method string

	hasBody bool
	body    []byte
}

func NewRequestSpec(
	path string,
	method string,
	rawQueryParams map[string][]string,
	hasBody bool,
	body []byte,
) *RequestSpec {
	query := make(map[string]map[string]int, len(rawQueryParams))

	for key, values := range rawQueryParams {
		freq := make(map[string]int, len(values))
		for _, v := range values {
			freq[v]++
		}
		query[key] = freq
	}

	normalized := strings.ToUpper(strings.TrimSpace(method))

	var bodyCopy []byte
	if hasBody {
		bodyCopy = append([]byte{}, body...)
	}

	return &RequestSpec{
		path:    path,
		method:  normalized,
		query:   query,
		hasBody: hasBody,
		body:    bodyCopy,
	}
}

func (r *RequestSpec) Equals(other *RequestSpec) bool {
	if r.method != other.method || r.path != other.path {
		return false
	}

	if len(r.query) != len(other.query) {
		return false
	}

	if r.hasBody != other.hasBody {
		return false
	}

	for key, rValues := range r.query {
		otherValues, ok := other.query[key]
		if !ok || len(rValues) != len(otherValues) {
			return false
		}

		for val, rCount := range rValues {
			if otherValues[val] != rCount {
				return false
			}
		}
	}

	if r.hasBody && !bytes.Equal(r.body, other.body) {
		return false
	}

	return true
}

func (r *RequestSpec) QueryParams() map[string][]string {
	out := make(map[string][]string, len(r.query))
	for key, freqMap := range r.query {
		values := make([]string, 0)
		for val, count := range freqMap {
			for i := 0; i < count; i++ {
				values = append(values, val)
			}
		}
		out[key] = values
	}

	return out
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

func (b *RequestSpecBuilder) Build() *RequestSpec {
	return NewRequestSpec(
		b.path,
		b.method,
		b.rawQueryParams,
		b.hasBody,
		b.body,
	)
}
