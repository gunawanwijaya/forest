package sdk_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"

	rest "github.com/gunawanwijaya/forest/sdk"
)

func test_HTTPMux(t *testing.T) {
	t.Parallel()

	Expect := NewWithT(t).Expect
	host := "http://example.com"
	root := host + "/"

	headerError := http.Header{}
	headerError.Set("Content-Type", "text/plain; charset=utf-8")
	headerError.Set("X-Content-Type-Options", "nosniff")

	code200, body200 := 200, []byte(http.StatusText(200))
	code404, body404 := 404, []byte(http.StatusText(404)+"\n")
	code500, body500 := 500, []byte(http.StatusText(500)+"\n")
	handle := func(statusCode int, header http.Header, body []byte) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(statusCode)
			for k, v := range header {
				for _, v := range v {
					w.Header().Add(k, v)
				}
			}
			_, _ = w.Write(body)
		})
	}
	handle200 := handle(code200, nil, body200)
	handle500 := handle(code500, headerError, body500)

	t.Run("middleware", func(t *testing.T) {
		w, r := newMockHandler("", root, nil)
		rest.Middleware(
			handle200,
			http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) { rest.CancelRequest(r) }),
			handle500,
		).ServeHTTP(w, r)
		Expect(testResponse(t, w, code200, nil, body200)).To(BeTrue())
	})
	t.Run("mux", func(t *testing.T) {
		t.Run("without-panic-handler", func(t *testing.T) {
			w, r := newMockHandler("", root, nil)
			new(rest.Mux).
				With(
					http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(0) }),
					rest.MuxMatcherMock(0, true, true)).
				ServeHTTP(w, r)
			Expect(testResponse(t, w, code500, headerError, body500)).To(BeTrue())
		})
		t.Run("without-notfound-handler", func(t *testing.T) {
			w, r := newMockHandler("", root, nil)
			new(rest.Mux).
				ServeHTTP(w, r)
			Expect(testResponse(t, w, code404, headerError, body404)).To(BeTrue())
		})
		t.Run("with-panic-handler", func(t *testing.T) {
			w, r := newMockHandler("", root, nil)

			mux := new(rest.Mux).
				With(
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic(99) }),
					rest.MuxMatcherMock(0, true, true)).
				With(
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic(99) }),
					rest.MuxMatcherMock(0, true, false))
			mux.PanicHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(rest.PanicRecoveryFromRequest(r)).To(Equal(99))
				handle500.ServeHTTP(w, r)
			})
			mux.ServeHTTP(w, r)

			Expect(testResponse(t, w, code500, headerError, body500)).To(BeTrue())
		})
		t.Run("with-notfound-handler", func(t *testing.T) {
			w, r := newMockHandler("", root, nil)
			mux := new(rest.Mux)
			mux.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handle500.ServeHTTP(w, r)
			})
			mux.ServeHTTP(w, r)
			Expect(testResponse(t, w, code500, headerError, body500)).To(BeTrue())
			Expect(testResponse(nil, w, code200, nil, body200)).To(BeFalse())
		})
	})
	t.Run("mock", func(t *testing.T) {
		w, r := newMockHandler("", root, nil)
		new(rest.Mux).
			With(handle200, rest.MuxMatcherOr(0,
				rest.MuxMatcherMock(0, true, true),
				rest.MuxMatcherMock(0, true, true),
			)).
			With(handle200, rest.MuxMatcherOr(0,
				rest.MuxMatcherMock(.1, true, true),
				rest.MuxMatcherMock(.2, true, true),
			)).
			ServeHTTP(w, r)
		Expect(testResponse(t, w, code200, nil, body200)).To(BeTrue())
	})
	t.Run("methods", func(t *testing.T) {
		t.Run("default", func(t *testing.T) {
			w, r := newMockHandler("", host+"/x/yyy/z", nil)
			new(rest.Mux).
				With(handle200, rest.MuxMatcherOr(0,
					rest.MuxMatcherMethods(0),
					rest.MuxMatcherMethods(0, "GET"),
					rest.MuxMatcherMethods(0,
						"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "CONNECT", "OPTIONS", "TRACE", "*"),
				)).
				ServeHTTP(w, r)
			Expect(testResponse(t, w, code200, nil, body200)).To(BeTrue())
		})
		t.Run("fail", func(t *testing.T) {
			w, r := newMockHandler("", host+"/x/yyy/z", nil)
			new(rest.Mux).
				With(nil, nil).
				With(handle200, rest.MuxMatcherMethods(0, "XXX")).
				With(handle200, nil).
				With(handle200, rest.MuxMatcherMethods(0, "GET")).
				ServeHTTP(w, r)
			Expect(testResponse(t, w, code200, nil, body200)).To(BeTrue())
		})
	})
	t.Run("pattern", func(t *testing.T) {
		t.Run("colon-start", func(t *testing.T) {
			w, r := newMockHandler("", host+"/x/yyy/z", nil)
			mux := new(rest.Mux).
				Handle("*", "/makan", handle200).
				Handle("*", "/:args1", handle200).
				Handle("*", "/:args1/:args2", handle200).
				Handle("GET", "/:args1/:args2/:args3", handle200)
			mux.ServeHTTP(w, r)
			Expect(testResponse(t, w, code200, nil, body200)).To(BeTrue())
			Expect(rest.NamedArgsFromRequest(r).Get("args1")).To(Equal("x"))
			Expect(rest.NamedArgsFromRequest(r).Get("args2")).To(Equal("yyy"))
			Expect(rest.NamedArgsFromRequest(r).Get("args3")).To(Equal("z"))
		})
		t.Run("colon-both", func(t *testing.T) {
			w, r := newMockHandler("", host+"/x/yyy/z", nil)
			mux := new(rest.Mux).
				With(handle200,
					rest.MuxMatcherPattern(0, "/:args1:/:args2:/:args3:", ":", ":", false))
			mux.ServeHTTP(w, r)
			Expect(testResponse(t, w, code200, nil, body200)).To(BeTrue())
			Expect(rest.NamedArgsFromRequest(r).Get("args1")).To(Equal("x"))
			Expect(rest.NamedArgsFromRequest(r).Get("args2")).To(Equal("yyy"))
			Expect(rest.NamedArgsFromRequest(r).Get("args3")).To(Equal("z"))
		})
		t.Run("double-curly-braces", func(t *testing.T) {
			w, r := newMockHandler("", host+"/x/yyy/z", nil)
			mux := new(rest.Mux).
				With(handle200,
					rest.MuxMatcherPattern(0, "/{{args1}}/{{args2}}/{{args3}}", "{{", "}}", false))
			mux.ServeHTTP(w, r)
			Expect(testResponse(t, w, code200, nil, body200)).To(BeTrue())
			Expect(rest.NamedArgsFromRequest(r).Get("args1")).To(Equal("x"))
			Expect(rest.NamedArgsFromRequest(r).Get("args2")).To(Equal("yyy"))
			Expect(rest.NamedArgsFromRequest(r).Get("args3")).To(Equal("z"))
		})
		t.Run("exact-no-pattern", func(t *testing.T) {
			w, r := newMockHandler("", host+"/x/yyy/z", nil)
			mux := new(rest.Mux).
				With(handle200,
					rest.MuxMatcherPattern(0, "/x/yyy/z", "", "", false))
			mux.ServeHTTP(w, r)
			Expect(testResponse(t, w, code200, nil, body200)).To(BeTrue())
			Expect(rest.NamedArgsFromRequest(r).Get("args1")).To(Equal(""))
			Expect(rest.NamedArgsFromRequest(r).Get("args2")).To(Equal(""))
			Expect(rest.NamedArgsFromRequest(r).Get("args3")).To(Equal(""))
		})
	})
	t.Run("test response", func(t *testing.T) {
		w, r := newMockHandler("", root, nil)
		Expect(w).NotTo(BeNil())
		Expect(r).NotTo(BeNil())
		new(rest.Mux).ServeHTTP(w, r)
		Expect(testResponse(t, w, code404, headerError, body404)).To(BeTrue())
	})
}

func newMockHandler(method, target string, body io.Reader) (*httptest.ResponseRecorder, *http.Request) {
	return httptest.NewRecorder(), httptest.NewRequest(method, target, body)
}

// testResponse check w so that it fulfill code, header & body accordingly.
func testResponse(tb testing.TB, w *httptest.ResponseRecorder, code int, header http.Header, body []byte) bool {
	if len(header) < 1 {
		header = http.Header{}
	}

	codeEq := w.Code == code
	bodyEq := bytes.Equal(w.Body.Bytes(), body)
	headerEq := len(w.Header()) == len(header)

	for k := range w.Header() {
		for i := range w.Header()[k] {
			headerEq = headerEq && i < len(header[k])
			headerEq = headerEq && len(header[k]) == len(w.Header()[k])
			headerEq = headerEq && header[k][i] == w.Header()[k][i]
		}
	}

	logf := func(string, ...interface{}) {}
	if tb != nil {
		logf = tb.Logf
	}

	if !headerEq {
		logf("Header:	\n\tExpect:	%s\n\tActual:	%s\n", header, w.Header())
	}

	if !codeEq {
		logf("Code:	\n\tExpect:	%d\n\tActual:	%d\n", code, w.Code)
	}

	if !bodyEq {
		logf("Body:	\n\tExpect:	%q\n\tActual:	%q\n", body, w.Body.Bytes())
	}

	return headerEq && codeEq && bodyEq
}
