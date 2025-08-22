// Package tests for DomainSentinel.
//
// File: deny_response_test.go
//
// What this file covers (UNIT TESTS):
//   - Rendering & writing of custom deny responses without running the full middleware.
//   - Verifies status codes, headers, and body produced by writeDeny/renderTemplateString.
//   - Checks templating helpers (urlquery, json, upper/lower), HTML vs. non-HTML rendering,
//     automatic Content-Type inference, and header templating.
//   - Ensures sensible defaults (fallback 403 text/plain) when no deny response is configured.
//
// Why unit tests here?
//   - They isolate the response/templating logic from path/IP matching and Traefik plumbing.
//   - Fast and deterministic: use httptest.ResponseRecorder to assert status/header/body.
//
// How to run:
//   go test -v ./...
//
// How to extend:
//   - Add a test for each new helper function or rendering rule.
//   - Add table-driven cases for new Content-Types or header keys you support.
//   - Keep these tests independent of request routing; that belongs in middleware_integration_test.go.

package DomainSentinel

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteDeny_Default403(t *testing.T) {
	rr := httptest.NewRecorder()

	writeDeny(rr, nil, nil, denyData{
		ClientIP: "1.2.3.4",
		Host:     "demo.localhost",
		Path:     "/admin",
	})

	if rr.Code != 403 {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("Content-Type = %q, want text/plain", ct)
	}
	if !strings.Contains(rr.Body.String(), "DS: Forbidden") {
		t.Fatalf("body = %q, want contains DS: Forbidden", rr.Body.String())
	}
}

func TestWriteDeny_HTMLAutoContentType(t *testing.T) {
	resp := &DenyResponse{
		Body: "<html><body>hi</body></html>",
	}
	rr := httptest.NewRecorder()
	writeDeny(rr, resp, nil, denyData{})

	if rr.Code != 403 {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}
}

func TestWriteDeny_HeaderAndBodyTemplating_Redirect(t *testing.T) {
	resp := &DenyResponse{
		StatusCode:  302,
		ContentType: "text/plain; charset=utf-8",
		Headers: map[string]string{
			"Location": "/login?next=[[ .Path | urlquery ]]",
			"ClientIP": "[[ .ClientIP ]]",
		},
		Body: "Redirecting…",
	}

	d := denyData{
		ClientIP: "1.2.3.4",
		Host:     "demo.localhost",
		Path:     "/admin/settings?tab=2",
	}

	rr := httptest.NewRecorder()
	writeDeny(rr, resp, nil, d)

	if rr.Code != 302 {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	if got, want := rr.Header().Get("Location"), "/login?next=%2Fadmin%2Fsettings%3Ftab%3D2"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
	if got, want := rr.Header().Get("ClientIP"), "1.2.3.4"; got != want {
		t.Fatalf("ClientIP header = %q, want %q", got, want)
	}
	if got := rr.Body.String(); !strings.Contains(got, "Redirecting") {
		t.Fatalf("body = %q, want contains Redirecting", got)
	}
}

func TestWriteDeny_JSONRendering_WithHelpers(t *testing.T) {
	resp := &DenyResponse{
		StatusCode:  429,
		ContentType: "application/json",
		Body:        `{"ip": [[ json .ClientIP ]], "host": [[ json .Host ]]}`,
	}

	d := denyData{ClientIP: "5.6.7.8", Host: "demo.localhost", Path: "/api"}
	rr := httptest.NewRecorder()
	writeDeny(rr, resp, nil, d)

	if rr.Code != 429 {
		t.Fatalf("status = %d, want 429", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var m map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("body not valid JSON: %v\n%s", err, rr.Body.String())
	}
	if m["ip"] != "5.6.7.8" || m["host"] != "demo.localhost" {
		t.Fatalf("json = %+v, want ip=5.6.7.8 host=demo.localhost", m)
	}
}

func TestWriteDeny_DomainFallback_WhenPathNil(t *testing.T) {
	domain := &DenyResponse{
		StatusCode: 503,
		// ContentType absichtlich leer lassen, Body ist HTML -> auto html
		Body: "<!doctype html><html><h1>Down</h1></html>",
	}
	rr := httptest.NewRecorder()
	writeDeny(rr, nil, domain, denyData{})

	if rr.Code != 503 {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
}
