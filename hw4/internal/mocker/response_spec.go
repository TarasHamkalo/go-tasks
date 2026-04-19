package mocker

type ResponseSpec struct {
	statusCode int

	hasBody bool
	body    []byte
}

func NewResponseSpec(statusCode int, hasBody bool, body []byte) *ResponseSpec {
	var bodyCopy []byte
	if body != nil {
		bodyCopy = append([]byte{}, body...)
	}
	return &ResponseSpec{statusCode: statusCode, hasBody: hasBody, body: bodyCopy}
}

func (r *ResponseSpec) HasBody() bool {
	return r.hasBody
}

func (r *ResponseSpec) StatusCode() int {
	return r.statusCode
}

func (r *ResponseSpec) Body() []byte {
	return append(make([]byte, 0, len(r.body)), r.body...)
}
