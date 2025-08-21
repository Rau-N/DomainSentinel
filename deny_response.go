package DomainSentinel

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	texttpl "text/template"
)

type DenyResponse struct {
	StatusCode  int               `json:"statusCode"  yaml:"statusCode"  mapstructure:"statusCode"`
	ContentType string            `json:"contentType" yaml:"contentType" mapstructure:"contentType"`
	Body        string            `json:"body"        yaml:"body"        mapstructure:"body"`
	Headers     map[string]string `json:"headers"     yaml:"headers"     mapstructure:"headers"`
}

type denyData struct {
	ClientIP    string
	Host        string
	Path        string
	MatchedRule string
}

func writeDeny(rw http.ResponseWriter, pathResp, domainResp *DenyResponse, data denyData) {
	resp := firstNonNil(pathResp, domainResp)
	if resp == nil {
		http.Error(rw, "DS: Forbidden", http.StatusForbidden)
		return
	}

	// set response code from config or default value
	code := resp.StatusCode
	if code == 0 {
		code = http.StatusForbidden
	}

	// automatically set value of Content-Type header based on body content
	ct := strings.TrimSpace(resp.ContentType)
	if ct == "" {
		if isStringHtml(resp.Body) {
			ct = "text/html; charset=utf-8"
		} else {
			ct = "text/plain; charset=utf-8"
		}
	}
	isHTML := strings.Contains(strings.ToLower(ct), "html")

	// set response headers from config
	for k, v := range resp.Headers {
		val := renderTemplateString(v, data, false)
		rw.Header().Set(k, sanitizeHeader(val))
	}

	rw.Header().Set("Content-Type", ct)

	// render data into body
	body := renderTemplateString(resp.Body, data, isHTML)

	rw.WriteHeader(code)
	_, _ = rw.Write([]byte(body))
}

func firstNonNil[T any](a, b *T) *T {
	if a != nil {
		return a
	}
	return b
}

func isStringHtml(s string) bool {
	x := strings.TrimSpace(strings.ToLower(s))
	return strings.HasPrefix(x, "<!doctype") || strings.HasPrefix(x, "<html") ||
		strings.HasPrefix(x, "<head") || strings.HasPrefix(x, "<body")
}

func renderTemplateString(text string, data any, isHTML bool) string {

	funcs := texttpl.FuncMap{
		"urlquery": url.QueryEscape,
		"json":     func(v any) string { b, _ := json.Marshal(v); return string(b) },
		"upper":    strings.ToUpper,
		"lower":    strings.ToLower,
	}

	// only render if there are delimiters
	if !(strings.Contains(text, "[[") && strings.Contains(text, "]]")) {
		return text
	}

	if isHTML {
		t, err := template.New("deny").Funcs(template.FuncMap(funcs)).Delims("[[", "]]").Parse(text)
		if err != nil {
			return text
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return text
		}
		return buf.String()
	}

	t, err := texttpl.New("deny").Funcs(funcs).Delims("[[", "]]").Parse(text)
	if err != nil {
		return text
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return text
	}
	return buf.String()
}
