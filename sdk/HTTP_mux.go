package sdk

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const (
	multiplierExactPattern int = 10
	multiplierNKeys        int = 2
)

// Mux holds a map of entries.
type Mux struct {
	entries []muxEntry

	// PanicHandler can access the error recovered via PanicRecoveryFromRequest,
	// PanicRecoveryFromRequest is a helper under httphandler package
	PanicHandler    http.Handler
	NotFoundHandler http.Handler
	Middleware      func(next http.Handler) http.Handler
}

// ServeHTTP implement http.Handler interface.
func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		_    http.Handler = m
		code int          = 0
	)

	if m.Middleware == nil {
		m.Middleware = func(next http.Handler) http.Handler { return next }
	}

	defer func() {
		// if rcv := recover(); rcv != nil {
		// 	if m.PanicHandler == nil {
		// 		code = http.StatusInternalServerError
		// 		m.PanicHandler = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// 			http.Error(w, http.StatusText(code), code)
		// 		})
		// 	}

		// 	set(r, ctxKeyPanicRecovery{}, rcv)
		// 	m.Middleware(m.PanicHandler).ServeHTTP(w, CancelRequest(r))
		// }
	}()

	var found bool

	for _, e := range m.entries {
		if e.matcher != nil && e.next != nil {
			if found = e.matcher.Match(r); found {
				m.Middleware(e.next).ServeHTTP(w, r)

				return
			}
		}
	}

	if !found {
		if m.NotFoundHandler == nil {
			code = http.StatusNotFound
			m.NotFoundHandler = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				http.Error(w, http.StatusText(code), code)
			})
		}

		m.Middleware(m.NotFoundHandler).ServeHTTP(w, CancelRequest(r))
	}
}

// With will register http.Handler with any implementation of MuxMatcher.
func (m *Mux) With(next http.Handler, matcher MuxMatcher) *Mux {
	PanicIf(next == nil, "next handler can not be nil")
	PanicIf(next == m, "next handler can not be mux itself")
	PanicIf(matcher == nil, "matcher can not be nil")
	PanicIf(!matcher.Test(), "test matcher failed")

	bm, _ := JSON.Marshal(matcher)

	for _, e := range m.entries {
		be, _ := JSON.Marshal(e.matcher)
		if bytes.Equal(be, bm) {
			return m
		}
	}

	m.entries = append(m.entries, muxEntry{next, matcher})
	sort.SliceStable(m.entries, func(i, j int) bool {
		ii := m.entries[i].matcher.Priority()
		jj := m.entries[j].matcher.Priority()

		return ii > jj
	})

	return m
}

// Handle will register http.Handler with MuxMatcherMethods on method and
// MuxMatcherPattern on pattern, see more details on each mux matcher
// implementation.
func (m *Mux) Handle(method string, pattern string, next http.Handler) *Mux {
	return m.With(next, MuxMatcherAnd(0,
		MuxMatcherMethods(0, method),
		MuxMatcherPattern(0, pattern, "", "", false),
	))
}

// muxEntry is an element of entries listed in mux.
type muxEntry struct {
	next    http.Handler
	matcher MuxMatcher
}

// MuxMatcher is an incoming *http.Request matcher.
type MuxMatcher interface {
	// Test is called when MuxMatcher is added to Mux, this should be an
	// opportunity to set priority and test the implementation parameter
	// e.g. MuxMatcherPattern should check pattern, start, end
	Test() bool

	// Priority is called after Test() returning true to set a priority queue
	Priority() float64

	// Match is called by order of Priority, after its turn, it will validate
	// the *http.Request and if the result is true, the http.Handler registered
	// in the entry will be served
	Match(*http.Request) bool
}

// -----------------------------------------------------------------------------
// MuxMatcherMock
// -----------------------------------------------------------------------------

type muxMatcherMock struct {
	P float64 `json:"priority"`
	T bool    `json:"test"`
	M bool    `json:"match"`
}

func MuxMatcherMock(priority float64, test, match bool) *muxMatcherMock {
	var _ MuxMatcher = (*muxMatcherMock)(nil)

	return &muxMatcherMock{priority, test, match}
}

func (m *muxMatcherMock) Test() bool { return m.T }

func (m *muxMatcherMock) Match(r *http.Request) bool { return m.M }

func (m *muxMatcherMock) Priority() float64 { return m.P }

// -----------------------------------------------------------------------------
// MuxMatcherOr
// -----------------------------------------------------------------------------

type muxMatcherOr struct {
	P     float64      `json:"priority"`
	Muxes []MuxMatcher `json:"muxes"`
}

func MuxMatcherOr(priority float64, muxes ...MuxMatcher) *muxMatcherOr {
	var _ MuxMatcher = (*muxMatcherOr)(nil)

	return &muxMatcherOr{priority, uniqueMuxMatcher(muxes)}
}

func (m *muxMatcherOr) Test() bool {
	match, p, c := false, 0.0, 0.0
	for i := range m.Muxes {
		p, c = p+m.Muxes[i].Priority(), c+1.0
		match = match || m.Muxes[i] != nil && m.Muxes[i].Test()
	}

	if m.P == 0 {
		m.P = p / c
	}

	return match
}

func (m *muxMatcherOr) Match(r *http.Request) bool {
	match := false

	for i := range m.Muxes {
		if m.Muxes[i] != nil && m.Muxes[i].Match(r) {
			match = true
		}
	}

	return match
}

func (m *muxMatcherOr) Priority() float64 { return m.P }

// -----------------------------------------------------------------------------
// MuxMatcherAnd
// -----------------------------------------------------------------------------

type muxMatcherAnd struct {
	P     float64      `json:"priority"`
	Muxes []MuxMatcher `json:"muxes"`
}

func MuxMatcherAnd(priority float64, muxes ...MuxMatcher) *muxMatcherAnd {
	var _ MuxMatcher = (*muxMatcherAnd)(nil)

	return &muxMatcherAnd{priority, uniqueMuxMatcher(muxes)}
}

func (m *muxMatcherAnd) Test() bool {
	match, p := false, 0.0

	for i := range m.Muxes {
		if m.Muxes[i] != nil {
			if i == 0 {
				match = true
			}

			match = match && m.Muxes[i].Test()
			p = p + m.Muxes[i].Priority()
		}
	}

	if m.P == 0 {
		m.P = p
	}

	return match
}

func (m *muxMatcherAnd) Match(r *http.Request) bool {
	match := false

	for i := range m.Muxes {
		if m.Muxes[i] != nil {
			if i == 0 {
				match = true
			}

			match = match && m.Muxes[i].Match(r)
		}
	}

	return match
}

func (m *muxMatcherAnd) Priority() float64 { return m.P }

// -----------------------------------------------------------------------------
// MuxMatcherMethods
// -----------------------------------------------------------------------------

type muxMatcherMethods struct {
	P       float64  `json:"priority"`
	Methods []string `json:"methods"`
}

// MuxMatcherMethods receive multiple methods, if contains asterisk `*` then
// the priority should be set to 0.
func MuxMatcherMethods(priority float64, methods ...string) *muxMatcherMethods {
	var _ MuxMatcher = (*muxMatcherMethods)(nil)

	sort.Strings(methods)

	return &muxMatcherMethods{priority, uniqueString(methods)}
}

func (m *muxMatcherMethods) Test() bool {
	if len(m.Methods) < 1 {
		return false
	}

	for i := range m.Methods {
		switch m.Methods[i] {
		case "*":
			m.P = 0

			return true
		case
			http.MethodGet,
			http.MethodHead,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodConnect,
			http.MethodOptions,
			http.MethodTrace:
			m.P = 1
			if l, max := float64(len(m.Methods)), 10.0; l < max {
				m.P = max - l
			}
		default:
			return false
		}
	}

	return true
}

func (m *muxMatcherMethods) Match(r *http.Request) bool {
	for i := range m.Methods {
		switch m.Methods[i] {
		case "*":
			return true
		case r.Method:
			return true
		}
	}

	return false
}

func (m *muxMatcherMethods) Priority() float64 { return m.P }

// -----------------------------------------------------------------------------
// MuxMatcherPattern
// -----------------------------------------------------------------------------

type muxMatcherPattern struct {
	// P scale with pattern
	P float64 `json:"priority"`

	// Pattern of named arguments using colon, e.g. `/:args1/:args2/:args3`
	Pattern string `json:"pattern"`

	// pair of Start & End token
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`

	CaseSensitive bool `json:"case_sensitive"`

	// parseURI receive path *http.Request.URL.Path and extract its values
	// according to a pattern supplied to url.Values, if path followed the
	// pattern it should return true
	parseURI func(string) (url.Values, bool) `json:"-"`

	tested  bool `json:"-"`
	testVal bool `json:"-"`
}

// MuxMatcherPattern receive pattern of named arguments using a pair of start
// and end string; if start is empty string, then assuming start is colon `:`,
// when end is empty string, then assuming end is slash `/`
//  `/:args1/:args2/:args3` // colon at start of arguments
//  `/:args1:/:args2:/:args3:` // colon at both start and end
//  `/{args1}/{args2}/{args3}` // curly-braces at both start and end
func MuxMatcherPattern(priority float64, pattern, start, end string, caseSensitive bool) *muxMatcherPattern {
	var _ MuxMatcher = (*muxMatcherPattern)(nil)

	if start == "" {
		start = ":"
	}

	if end == "" {
		end = "/"
	}

	return &muxMatcherPattern{priority, pattern, start, end, caseSensitive, nil, false, false}
}

type patternKey struct {
	int
	string
}

func (m *muxMatcherPattern) parsePattern() (pat string, keys []patternKey, l int) {
	s, b := make([]rune, 0), new(strings.Builder)
	found, skipFound, skipChar, set := false, 0, 0, map[string]struct{}{}
	lookahead := func(src, sub string, i int) bool {
		return i+len(sub) <= len(src) && src[i:i+len(sub)] == sub
	}

	if !m.CaseSensitive {
		m.Pattern = strings.ToLower(m.Pattern)
		m.Start = strings.ToLower(m.Start)
		m.End = strings.ToLower(m.End)
	}

	for i, e := range m.Pattern {
		if skipChar > 0 {
			skipChar--

			continue
		}

		if !found && lookahead(m.Pattern, m.Start, i) {
			keys, s = append(keys, patternKey{0, string(s)}), make([]rune, 0)
			skipChar, found = len(m.Start)-1, true

			continue
		}

		s = append(s, e)

		isEnd, isLastChar := lookahead(m.Pattern, m.End, i), i == len(m.Pattern)-1
		if found && (isEnd || isLastChar) {
			skipChar, found = len(m.End)-1, false

			if i > skipFound && i-skipFound > 0 {
				j := i
				if !isEnd && isLastChar {
					j++
				}

				key := m.Pattern[i-skipFound : j]
				keys, s = append(keys, patternKey{1, key}), make([]rune, 0)
				skipFound, set[key] = 0, struct{}{}
				_, _ = b.WriteString(`%s`)

				if m.End == "/" {
					s = append(s, '/')
				}

				if lookahead(m.Pattern, m.Start, i+1) || (m.End == "/" && !isLastChar) {
					_, _ = b.WriteString(m.End)
				}
			}

			continue
		}

		if found {
			skipFound++
		} else {
			skipFound, _ = 0, b.WriteByte(m.Pattern[i])
		}
	}

	return b.String(), keys, len(keys)
}

func (m *muxMatcherPattern) Test() bool {
	if m.tested {
		return m.testVal
	}

	m.tested = true
	if len(m.Pattern) < 1 {
		m.testVal = false

		return m.testVal
	}

	pat, keys, l := m.parsePattern()
	hook := func(src string) {}

	if l < 1 { // when no key found, it's the exact match
		m.Start, m.End = "", ""
		m.P = float64(len(m.Pattern) * multiplierExactPattern)
		m.parseURI = func(uri string) (url.Values, bool) {
			hook(uri)

			return nil, uri == m.Pattern
		}
		m.testVal = true

		return m.testVal
	}

	if m.P == 0 { // auto-assign priority
		m.P = float64((len(pat) * multiplierExactPattern) + (l * multiplierNKeys))
	}

	m.parseURI = func(uri string) (u url.Values, match bool) {
		hook(uri)

		u = make(url.Values, 0)

		for i, key := range keys {
			switch key.int {
			case 0:
				if !strings.HasPrefix(uri, key.string) {
					return nil, false
				}

				uri = uri[len(key.string):]
			case 1:
				if i < len(keys)-1 {
					idx, nextS, nextI := 0, keys[i+1].string, keys[i+1].int
					switch nextI {
					case 0:
						idx = strings.Index(uri, nextS)

					case 1:
						idx = strings.Index(uri, m.End)
						u.Set(key.string, uri[:idx])
					}

					if idx < 0 {
						return nil, false
					}

					uri = uri[idx:]
				} else {
					u.Set(key.string, uri)
				}
			}
		}

		return u, len(u) > 0
	}

	m.testVal = true

	return m.testVal
}

func (m *muxMatcherPattern) Match(r *http.Request) bool {
	uri := r.URL.Path
	if !m.CaseSensitive {
		uri = strings.ToLower(uri)
	}

	u, match := m.parseURI(uri)
	if match && len(u) > 0 {
		set(r, ctxKeyNamedArgs{}, u)
	}

	return match
}

func (m *muxMatcherPattern) Priority() float64 { return m.P }

func uniqueMuxMatcher(muxes []MuxMatcher) (nMuxes []MuxMatcher) {
	for i := range muxes {
		_ = muxes[i].Test()
		bi, _ := JSON.Marshal(muxes[i])
		skip := false

		for j := range muxes {
			_ = muxes[j].Test()
			bj, _ := JSON.Marshal(muxes[j])
			skip = i != j && (skip || bytes.Equal(bi, bj))
		}

		if !skip {
			nMuxes = append(nMuxes, muxes[i])
		}
	}

	return nMuxes
}

func uniqueString(strs []string) (nStrs []string) {
	for i := range strs {
		skip := false
		for j := range strs {
			skip = i != j && (skip || strs[i] == strs[j])
		}

		if !skip {
			nStrs = append(nStrs, strs[i])
		}
	}

	return nStrs
}

// NamedArgsFromRequest is a helper function that extract url.Values that have
// been parsed using MuxMatcherPattern, url.Values should not be empty if
// parsing is successful and should be able to extract further following
// url.Values, same keys in the pattern result in new value added in url.Values.
func NamedArgsFromRequest(r *http.Request) url.Values {
	u, _ := get(r, ctxKeyNamedArgs{}).(url.Values)

	return u
}

// PanicRecoveryFromRequest is a helper function that extract error value
// when panic occurred, the value is saved to *http.Request after recovery
// process and right before calling mux.PanicHandler.
func PanicRecoveryFromRequest(r *http.Request) interface{} {
	return get(r, ctxKeyPanicRecovery{})
}

// CancelRequest will cancel the underlying context from *http.Request.
func CancelRequest(r *http.Request) *http.Request {
	ctx, cancel := context.WithCancel(r.Context())
	cancel()

	*r = *(r.WithContext(ctx))

	return r
}

// Middleware create a stack of http.Handler that is cancelable via CancelRequest.
func Middleware(h ...http.Handler) http.Handler { return middleware(h) }

type middleware []http.Handler

func (mw middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for i := range mw {
		select {
		case <-r.Context().Done():
			break
		default:
			mw[i].ServeHTTP(w, r)
		}
	}
}

func set(r *http.Request, key, val interface{}) {
	*r = *(r.WithContext(context.WithValue(r.Context(), key, val)))
}

func get(r *http.Request, key interface{}) interface{} {
	return r.Context().Value(key)
}

type ctxKeyNamedArgs struct{}

type ctxKeyPanicRecovery struct{}
