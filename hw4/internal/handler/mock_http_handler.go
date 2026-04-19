package handler

import (
	"http-mocker/internal/middleware"
	"http-mocker/internal/mocker"
	"io"
	"net/http"

	"go.uber.org/zap"
)

type MockHttpHandler struct {
	mocker *mocker.HttpMocker
	logger *zap.Logger
}

func NewMockHttpHandler(mocker *mocker.HttpMocker, logger *zap.Logger) *MockHttpHandler {
	return &MockHttpHandler{mocker: mocker, logger: logger}
}

func (m *MockHttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestSpec, err := m.parseRequest(r)
	if err != nil {
		m.logger.Error(
			"failed to read request body",
			zap.String("trace", middleware.GetTraceId(r.Context())),
			zap.Error(err),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	responseSpec, err := m.mocker.Serve(requestSpec)
	if err != nil {
		statusCode := m.mapToStatusCode(err)
		m.logger.Error(
			"failed to serve request",
			zap.String("trace", middleware.GetTraceId(r.Context())),
			zap.Int("statusCode", statusCode),
			zap.Error(err),
		)

		w.WriteHeader(statusCode)
		return
	}

	responseBody := responseSpec.Body() // allocates copy
	m.logger.Info(
		"serve request",
		zap.String("trace", middleware.GetTraceId(r.Context())),
		zap.Int("statusCode", responseSpec.StatusCode()),
		// TODO: remove, should not be used normally, but for testing with small bodies, it is nice
		zap.ByteString("body", responseBody),
	)

	w.WriteHeader(responseSpec.StatusCode())
	if responseSpec.HasBody() {
		_, err = w.Write(responseBody)
		if err != nil {
			m.logger.Error(
				"failed to write response",
				zap.String("trace", middleware.GetTraceId(r.Context())),
				zap.Error(err),
			)
		}
	}
}

func (m *MockHttpHandler) parseRequest(r *http.Request) (*mocker.RequestSpec, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	m.logger.Debug(
		"request body received",
		zap.String("trace", middleware.GetTraceId(r.Context())),
	)

	// TODO: can be enforced by checking presence of Content-Length,
	//  though clients can send it even when body not specified, for demo leaving like this
	hasBody := len(bodyBytes) > 0
	requestSpec := mocker.NewRequestSpec(
		r.URL.EscapedPath(), r.Method, r.URL.Query(), hasBody, bodyBytes,
	)

	return requestSpec, nil
}

func (m *MockHttpHandler) mapToStatusCode(err error) int {
	switch err {
	case mocker.ErrNoConfigurationExists:
		return http.StatusNotImplemented
	case mocker.ErrMethodNotSupported:
		return http.StatusMethodNotAllowed
	case
		mocker.ErrSpecificationDiffers,
		mocker.ErrMethodNotRegistered,
		mocker.ErrPathNotRegistered:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
