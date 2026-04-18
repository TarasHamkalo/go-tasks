package mocker

type ResponseSpec struct {
	statusCode int
	body       []byte
}

func NewResponseSpec(statusCode int, body []byte) *ResponseSpec {
	return &ResponseSpec{statusCode: statusCode, body: body}
}

func (r ResponseSpec) StatusCode() int {
	return r.statusCode
}

func (r ResponseSpec) Body() []byte {
	return r.body
}
