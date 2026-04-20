package mocker

import (
	"bytes"
	"strings"
)

// RequestSpec represents HTTP request accepted by HttpMocker.
// Immutable after creation, copy on read for query/body.
type RequestSpec struct {
	// path is normalized URL path (without parameters)
	path string

	// query stores URL query parameters
	// in form of association name -> (value, frequency)
	// NOTE: initially used set of "key:value:freq" elements, but hard to parse back
	query map[string]map[string]int

	// method represents HTTP method,
	// validation of method type is left to creator of this object
	// e.g. HttpMocker during SetRoute
	method string

	// hasBody indicates whether request hasBody,
	// used to always store non-nil value of body field
	hasBody bool

	// body represents request body, always non-nil
	body []byte
}

// NewRequestSpec constructs new RequestSpec object,
// all mutable fields are cloned
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

// Equals verifies whether provided request is logically equivalent,
// ignoring query parameter order.
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

// QueryParams returns query parameters in form used by URL.Query.
// Each time new copy is created.
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

// Body returns new copy of request body.
func (r *RequestSpec) Body() []byte {
	return append([]byte{}, r.body...)
}

func (r *RequestSpec) Method() string {
	return r.method
}

func (r *RequestSpec) Path() string {
	return r.path
}
