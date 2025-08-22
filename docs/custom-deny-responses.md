# Custom Deny Responses

DomainSentinel can return **custom responses** when access is denied—at **path** level or **domain** level.  
You can choose the **status code**, **headers** (including `Location` for redirects), **content type**, and **body** (HTML/JSON/plain).  
Templating is supported in both headers and body.

> Precedence: **path-level `denyResponse` > domain-level `denyResponse` > default `403 Forbidden`**.

---

## Quick start

```yaml
http:
  middlewares:
    domain-sentinel:
      plugin:
        domainSentinel:
          domainPathRules:
            "demo.localhost":
              sourceIPs:
                - "0.0.0.0/0"
              pathRules:
                - path: "/admin/*"
                  sourceIPs:
                    - "10.10.4.0/24"
                  denyResponse:
                    statusCode: 302
                    headers:
                      Location: "/login?next=[[ .RequestURI | urlquery ]]"
                    contentType: "text/plain; charset=utf-8"
                    body: "Redirecting to login…"
              denyResponse:
                statusCode: 503
                headers:
                  ClientIP: "[[ .ClientIP ]]"
                contentType: "text/html; charset=utf-8"
                body: |
                  <!doctype html>
                  <html><h1>Service unavailable for [[ .Host ]]</h1></html>
```

---

## Configuration reference

### `denyResponse` object

| Field        | Type               | Required | Notes |
|--------------|--------------------|---------:|-------|
| `statusCode` | `int`              |    no    | Defaults to `403` if omitted. |
| `contentType`| `string`           |    no    | If omitted, it’s inferred from the body (`text/html` if the body looks like HTML, otherwise `text/plain`). Can be templated. |
| `headers`    | `map[string]string`|    no    | All header values support templating (see below). CR/LF are stripped for safety. |
| `body`       | `string`           |    no    | Supports templating. Rendered with HTML auto-escaping if `contentType` is HTML. |

**Where you can set it**

- **Domain level**: `domainPathRules["<host>"].denyResponse` (fallback for the whole host)
- **Path level**: `domainPathRules["<host>"].pathRules[i].denyResponse` (takes precedence)

---

## Templating

DomainSentinel uses Go templates with **`[[ ... ]]` delimiters** to avoid collisions with Traefik’s own `{{ ... }}` templating.

### Available data fields

- `[[ .ClientIP ]]` — detected client IP (by default from `RemoteAddr`)
- `[[ .Host ]]` — requested host (without port)
- `[[ .Path ]]` — URL path only (e.g. `/admin/panel`)
- `[[ .RawQuery ]]` — raw query string (e.g. `tab=2`)
- `[[ .RequestURI ]]` — path **plus** query (e.g. `/admin/panel?tab=2`)
- `[[ .MatchedRule ]]` — the path rule that matched (if any)

### Helper functions

- `urlquery` — URL-encodes strings for query parameters  
  `[[ .RequestURI | urlquery ]]  ->  %2Fadmin%2Fpanel%3Ftab%3D2`
- `json` — JSON-quotes a value (useful for JSON bodies)  
  `[[ json .Path ]]  ->  "/admin/panel"`
- `upper`, `lower` — case helpers

### Header & body rendering

- **Headers** are rendered with the text templating engine (no HTML escaping), e.g.:
  ```yaml
  headers:
    Location: "/login?next=[[ .RequestURI | urlquery ]]"
    ClientIP: "[[ .ClientIP ]]"
  ```
- **Body** is rendered:
  - with **HTML auto-escaping** if `contentType` contains `html`
  - otherwise with plain text templating (no HTML escaping)

---

## Examples

### 1) Pretty HTML page (451 Unavailable for Legal Reasons)

```yaml
denyResponse:
  statusCode: 451
  contentType: "text/html; charset=utf-8"
  headers:
    Cache-Control: "no-store"
  body: |
    <!doctype html>
    <html>
      <head><meta charset="utf-8"><title>No access</title></head>
      <body style="font-family:system-ui;margin:2rem">
        <h1>Access denied</h1>
        <p>IP: [[ .ClientIP ]]</p>
        <p>Host: [[ .Host ]]</p>
        <p>Path: [[ .Path ]]</p>
      </body>
    </html>
```

### 2) Redirect (302) to login, preserving original target

```yaml
denyResponse:
  statusCode: 302
  headers:
    Location: "/login?next=[[ .RequestURI | urlquery ]]"
  contentType: "text/plain; charset=utf-8"
  body: "Redirecting…"
```

> Prefer **303 See Other** to force GET after POST, or **307/308** to preserve the request method.

### 3) JSON API error (429)

```yaml
denyResponse:
  statusCode: 429
  headers:
    Retry-After: "60"
  contentType: "application/json"
  body: |
    {"error":"access_denied","ip": [[ json .ClientIP ]],"path": [[ json .Path ]]}
```

---

## Security notes

- **Client IP source**: By default, DomainSentinel uses `RemoteAddr` and **ignores `X-Forwarded-For`** to prevent spoofing.  
  If you operate behind trusted proxies and want to honor forward headers, implement/enable a trusted-proxy mode explicitly.

- **Header hygiene**: The middleware strips CR/LF from rendered header values.

- **Traefik templating**: Use `[[ ... ]]` delimiters. If you use `{{ ... }}` in dynamic file provider configs, Traefik will try to render them at load time and fail.

- **Caching**: For deny pages, consider `Cache-Control: no-store`.

---

## Test coverage

Two test layers exist in this repository:

- **Unit tests** (`deny_response_test.go`)  
  Validate response rendering (status, headers, body), templating helpers, and content-type inference.

- **Integration tests** (`middleware_integration_test.go`)  
  Exercise the middleware end-to-end: config → `New` → `ServeHTTP`, including precedence (path vs. domain) and pass-through to the next handler.

Run all tests:

```bash
go test -v ./...
```

---

## Troubleshooting

- **Traefik error like**: `can't evaluate field ClientIP in type bool`  
  You used `{{ .ClientIP }}` in YAML. Switch to `[[ .ClientIP ]]`.

- **Query missing in redirect**  
  Use `[[ .RequestURI | urlquery ]]` (includes path **and** query), not just `.Path`.

- **Body renders literally `[[ ... ]]`**  
  Ensure your strings actually contain `[[` and `]]` and that the plugin version includes custom templating.

---

## Changelog (feature)

- Add `denyResponse` at path/domain level (status, headers, body, contentType)
- Support templating with `[[ ... ]]` in **both** headers and body
- Helpers: `urlquery`, `json`, `upper`, `lower`
- Auto content-type detection for convenience
- Precedence: path > domain > default 403
