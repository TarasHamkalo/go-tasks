package endpoints

import (
	"bytes"
	"http-mocker/internal/mocker"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestGetUsersOK(t *testing.T) {
	logger := zap.NewNop()

	m := mocker.NewHttpMocker()
	m.SetRoute(
		mocker.NewRequestSpecBuilder("/users", "GET").Build(),
		mocker.NewResponseSpec(
			200,
			[]byte(`["user-1","user-2"]`),
		),
	)

	frontend := NewMockedHttpServer(m, logger)
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()

	frontend.httpSrv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	expected := `["user-1","user-2"]`
	if rec.Body.String() != expected {
		t.Fatalf("expected body %s, got %s", expected, rec.Body.String())
	}
}

func TestNoConfigReturns501(t *testing.T) {
	m := mocker.NewHttpMocker()
	frontend := NewMockedHttpServer(m, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()

	frontend.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", rec.Code)
	}
}

func TestNoSupportedMethodReturns405(t *testing.T) {
	m := mocker.NewHttpMocker()
	frontend := NewMockedHttpServer(m, zap.NewNop())

	req := httptest.NewRequest(http.MethodDelete, "/users", nil)
	rec := httptest.NewRecorder()

	frontend.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestPathNotFoundReturns404(t *testing.T) {
	m := mocker.NewHttpMocker()

	m.SetRoute(
		mocker.NewRequestSpecBuilder("/users", "GET").Build(),
		mocker.NewResponseSpec(200, []byte("ok")),
	)

	frontend := NewMockedHttpServer(m, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/invalid", nil)
	rec := httptest.NewRecorder()

	frontend.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestMethodDiffersReturns404(t *testing.T) {
	m := mocker.NewHttpMocker()

	m.SetRoute(
		mocker.NewRequestSpecBuilder("/users", "GET").Build(),
		mocker.NewResponseSpec(200, []byte("ok")),
	)

	frontend := NewMockedHttpServer(m, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	rec := httptest.NewRecorder()

	frontend.httpSrv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestPostBodyMatchOK(t *testing.T) {
	m := mocker.NewHttpMocker()

	m.SetRoute(
		mocker.NewRequestSpecBuilder("/test", "POST").
			WithBody([]byte("aaa")).
			Build(),
		mocker.NewResponseSpec(200, []byte("ok")),
	)

	frontend := NewMockedHttpServer(m, zap.NewNop())

	req := httptest.NewRequest(
		http.MethodPost,
		"/test",
		bytes.NewBufferString("aaa"),
	)
	rec := httptest.NewRecorder()

	frontend.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestPostBodyMismatchReturns404(t *testing.T) {
	m := mocker.NewHttpMocker()

	m.SetRoute(
		mocker.NewRequestSpecBuilder("/test", "POST").
			WithBody([]byte("aaa")).
			Build(),
		mocker.NewResponseSpec(200, []byte("ok")),
	)

	frontend := NewMockedHttpServer(m, zap.NewNop())

	req := httptest.NewRequest(
		http.MethodPost,
		"/test",
		bytes.NewBufferString("bbb"),
	)
	rec := httptest.NewRecorder()

	frontend.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestQueryOrderIgnored(t *testing.T) {
	m := mocker.NewHttpMocker()

	m.SetRoute(
		mocker.NewRequestSpecBuilder("/test", "GET").
			WithQueryParams(map[string][]string{
				"name": {"A", "B"},
			}).
			Build(),
		mocker.NewResponseSpec(200, []byte("ok")),
	)

	frontend := NewMockedHttpServer(m, zap.NewNop())

	req := httptest.NewRequest(
		http.MethodGet,
		"/test?name=B&name=A",
		nil,
	)
	rec := httptest.NewRecorder()

	frontend.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestQueryMultipleIdenticalKvPairsAccepted(t *testing.T) {
	m := mocker.NewHttpMocker()

	m.SetRoute(
		mocker.NewRequestSpecBuilder("/test", "GET").
			WithQueryParams(map[string][]string{
				"name": {"A", "A"},
			}).
			Build(),
		mocker.NewResponseSpec(200, []byte("ok")),
	)

	frontend := NewMockedHttpServer(m, zap.NewNop())

	req := httptest.NewRequest(
		http.MethodGet,
		"/test?name=A&name=A",
		nil,
	)
	rec := httptest.NewRecorder()

	frontend.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestQueryMismatchReturns404(t *testing.T) {
	m := mocker.NewHttpMocker()

	m.SetRoute(
		mocker.NewRequestSpecBuilder("/test", "GET").
			WithQueryParams(map[string][]string{
				"name": {"A", "B"},
			}).
			Build(),
		mocker.NewResponseSpec(200, []byte("ok")),
	)

	frontend := NewMockedHttpServer(m, zap.NewNop())

	req := httptest.NewRequest(
		http.MethodGet,
		"/test?name=A",
		nil,
	)
	rec := httptest.NewRecorder()

	frontend.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
