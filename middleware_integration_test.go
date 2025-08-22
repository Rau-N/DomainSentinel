// Package tests for DomainSentinel.
//
// File: middleware_integration_test.go
//
// What this file covers (INTEGRATION / COMPONENT TESTS):
//   - Runs the middleware end-to-end in-process: Config -> New(...) -> ServeHTTP.
//   - Exercises domain/path matching, IP allow/deny decisions, and the connection to
//     the deny-response renderer (status, headers, templated body).
//   - Verifies precedence (path-level deny overrides domain-level deny) and that
//     allowed requests pass through to the next handler.
//
// Test technique:
//   - Uses httptest.NewRequest and httptest.NewRecorder to simulate HTTP traffic.
//   - A small "next" handler (okNext) marks successful pass-through via a header.
//   - Client IP is taken from req.RemoteAddr (by design); tests set it explicitly.
//
// How to run:
//   go test -v ./...
//
// How to extend:
//   - Add cases for additional matching rules, more complex denyResponse configs,
//     and alternative redirect codes (307/308/303).
//   - Keep environment-local: these tests do not spin up Traefik; they validate the
//     middleware behavior in isolation from the proxy process.

package DomainSentinel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// helper next-handler to verify if the request got passed through
func okNext() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Next-Called", "true")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("OK"))
	})
}

func TestServeHTTP_PathDeny_TakesPrecedence(t *testing.T) {
	cfg := &Config{
		DomainPathRules: map[string]DomainConfig{
			"demo.localhost": {
				// Domain allows all ip addresses -> Path-Rule will decide
				SourceIPs: []string{"0.0.0.0/0"},
				PathRules: []PathConfig{
					{
						Path:      "/admin/*",
						SourceIPs: []string{"10.10.4.0/24"}, // test-IP not allowed -> deny
						Deny: &DenyResponse{
							StatusCode: 302,
							Headers: map[string]string{
								"Location": "/login?next=[[ .RequestURI | urlquery ]]",
							},
							ContentType: "text/plain; charset=utf-8",
							Body:        "Redirecting…",
						},
					},
				},
			},
		},
	}
	ds, err := New(context.Background(), okNext(), cfg, "test")
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	req := httptest.NewRequest("GET", "http://demo.localhost/admin/panel?tab=2", nil)
	req.RemoteAddr = "192.168.1.50:12345" // simulate client source IP
	rr := httptest.NewRecorder()

	ds.ServeHTTP(rr, req)

	if rr.Code != 302 {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	if rr.Header().Get("X-Next-Called") != "" {
		t.Fatalf("next handler should not be called on deny")
	}
	if got, want := rr.Header().Get("Location"), "/login?next=%2Fadmin%2Fpanel%3Ftab%3D2"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestServeHTTP_DomainDeny_WhenNoPathMatch(t *testing.T) {
	cfg := &Config{
		DomainPathRules: map[string]DomainConfig{
			"demo.localhost": {
				SourceIPs: []string{"10.10.3.0/24"}, // test-IP does not match -> domain deny
				PathRules: []PathConfig{
					{
						Path:      "/only/*",
						SourceIPs: []string{"10.0.0.0/8"}, // does not match path
					},
				},
				Deny: &DenyResponse{
					StatusCode: 503,
					Headers: map[string]string{
						"ClientIP": "[[ .ClientIP ]]",
					},
					Body: "<html>Down</html>", // ContentType empty -> auto html
				},
			},
		},
	}
	ds, err := New(context.Background(), okNext(), cfg, "test")
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	req := httptest.NewRequest("GET", "http://demo.localhost/other", nil)
	req.RemoteAddr = "192.168.1.50:12345" // simulate client source IP
	rr := httptest.NewRecorder()

	ds.ServeHTTP(rr, req)

	if rr.Code != 503 {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}
	if got := rr.Header().Get("ClientIP"); got != "192.168.1.50" {
		t.Fatalf("ClientIP header = %q, want 192.168.1.50", got)
	}
	if rr.Header().Get("X-Next-Called") != "" {
		t.Fatalf("next should not be called")
	}
}

func TestServeHTTP_Allowed_PassesToNext(t *testing.T) {
	cfg := &Config{
		DomainPathRules: map[string]DomainConfig{
			"demo.localhost": {
				SourceIPs: []string{"0.0.0.0/0"}, // allowed
				PathRules: []PathConfig{
					{
						Path:      "/admin/*",
						SourceIPs: []string{"0.0.0.0/0"}, // allowed
					},
				},
			},
		},
	}
	ds, err := New(context.Background(), okNext(), cfg, "test")
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	req := httptest.NewRequest("GET", "http://demo.localhost/admin", nil)
	req.RemoteAddr = "192.168.1.50:12345" // simulate client source IP
	rr := httptest.NewRecorder()

	ds.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if rr.Header().Get("X-Next-Called") != "true" {
		t.Fatalf("next not called")
	}
}
