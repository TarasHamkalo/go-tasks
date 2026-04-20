package mocker

type ResponseSpec struct {
	statusCode int

	headers map[string][]string

	hasBody bool
	body    []byte
}

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

func (r *ResponseSpec) Body() []byte {
	return append(make([]byte, 0, len(r.body)), r.body...)
}
