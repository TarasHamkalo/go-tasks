package mocker

// ResponseSpec represents HTTP response accepted by HttpMocker.
// Immutable after creation, copy on read for headers/body.
type ResponseSpec struct {
	// statusCode represents HTTP status code returned to client
	statusCode int

	// headers represents HTTP headers parsed to map (name, values array)
	headers map[string][]string

	// hasBody indicates whether response hasBody,
	// used to always store non-nil value of body field
	hasBody bool

	// body represents response body, always non-nil
	body []byte
}

// NewResponseSpec constructs new ResponseSpec object,
// all mutable fields are cloned.
func NewResponseSpec(
	statusCode int,
	headers map[string][]string,
	hasBody bool,
	body []byte,
) *ResponseSpec {
	headersCopy := make(map[string][]string, len(headers))
	for key, values := range headers {
		valuesCopy := make([]string, 0, len(values))
		for _, value := range values {
			valuesCopy = append(valuesCopy, value)
		}
		headersCopy[key] = valuesCopy
	}

	var bodyCopy []byte
	if body != nil {
		bodyCopy = append([]byte{}, body...)
	}

	return &ResponseSpec{
		statusCode: statusCode,
		headers:    headersCopy,
		hasBody:    hasBody,
		body:       bodyCopy,
	}
}

func (r *ResponseSpec) HasBody() bool {
	return r.hasBody
}

func (r *ResponseSpec) StatusCode() int {
	return r.statusCode
}

// Headers returns new copy of stored headers field.
func (r *ResponseSpec) Headers() map[string][]string {
	headersCopy := make(map[string][]string, len(r.headers))
	for key, values := range r.headers {
		valuesCopy := make([]string, 0, len(values))
		for _, value := range values {
			valuesCopy = append(valuesCopy, value)
		}
		headersCopy[key] = valuesCopy
	}
	return headersCopy
}

// Body returns new copy of response body.
func (r *ResponseSpec) Body() []byte {
	return append(make([]byte, 0, len(r.body)), r.body...)
}
