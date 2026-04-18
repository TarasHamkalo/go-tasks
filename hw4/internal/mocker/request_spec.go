package mocker

import (
	"bytes"
	"fmt"
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
		method:      method,
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
