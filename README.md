# Domain Sentinel

## Overview

The `domainSentinel` plugin is a Traefik middleware designed to **centrally manage access control based on source IP addresses**, organized by domain and URL path. Instead of configuring access lists individually on routers, this plugin allows you to define and enforce those rules **in one central location within Traefik**.

It provides fine-grained control by allowing both **domain-wide** and **path-specific** whitelisting using individual IP addresses and CIDR blocks. This is especially useful for protecting administrative interfaces, staging environments, internal APIs, or other sensitive routes, ensuring only trusted sources can reach them.

![Domain Sentinel diagram](.assets/domain_sentinel_diagram.excalidraw.svg)

## Structs and Configuration Explanation

### 1. `Config` Struct

**Purpose**: Holds the entire plugin configuration, mapping domain names to their respective access rules.

**Fields**:

- `DomainPathRules`
  - **Type**: `map[string]DomainConfig`
  - **Description**: Maps each domain name to a `DomainConfig` struct, which contains access rules for that domain and its paths.

---

### 2. `DomainConfig` Struct

**Purpose**: Contains source IP allowlists for a domain and its individual path rules.

**Fields**:

- `SourceIPs`
  - **Type**: `[]string`
  - **Description**: List of IP addresses or CIDR blocks that are allowed to access the domain globally.
  - **Example**:
    ```yaml
    sourceIPs:
      - "192.168.1.0/24"
      - "10.0.0.1"
    ```

- `PathRules`
  - **Type**: `[]PathConfig`
  - **Description**: A list of access rules that apply to specific URL paths within the domain. If a request path matches a rule, its corresponding IPs override the domain-wide list.
  - **Example**:
    ```yaml
    pathRules:
      - path: "/admin/*"
        sourceIPs:
          - "10.0.0.0/24"
          - "203.0.113.1"
    ```

---

### 3. `PathConfig` Struct

**Purpose**: Defines access rules for a specific path pattern.

**Fields**:

- `Path`
  - **Type**: `string`
  - **Description**: The URL path to protect. Supports exact match or wildcard prefix (`/path/*`).
  - **Example**: `"/admin/*"`

- `SourceIPs`
  - **Type**: `[]string`
  - **Description**: List of allowed IPs or CIDRs for this specific path. If a path rule matches, only these IPs are used to validate the request.
  - **Example**:
    ```yaml
    sourceIPs:
      - "192.168.100.1"
      - "10.0.0.0/24"
    ```

---

## How it Works

### Middleware Flow

1. **Extracts the domain** from the `Host` header (ignoring any port).
2. **Looks up the domain config** in `DomainPathRules`.
   - If no config is found → the request is **allowed**.
3. **Checks path-specific rules**:
   - If any rule’s `Path` matches the request URL:
     - Validates the request's source IP against the rule’s `SourceIPs`.
     - If the IP is allowed → the request proceeds.
     - Else → the request is blocked with `403 Forbidden`.
4. **Fallback to domain-wide IP rules** if no path rule matches.
   - Same logic applies using the `DomainConfig.SourceIPs`.

---

### IP Matching

- Supports both **individual IPs** (`203.0.113.5`) and **CIDR blocks** (`192.168.0.0/24`).
- The client IP is derived from `req.RemoteAddr` using `net.SplitHostPort`.
- All IP checks are done using Go's built-in `net.ParseIP` and `net.ParseCIDR`.

---

### Path Matching

- Paths can either:
  - Match exactly: `/admin`
  - Use a wildcard: `/admin/*` matches `/admin/`, `/admin/settings`, etc.
- If multiple path rules are defined, the **first matching rule wins**.

---

## Example Configuration

```yaml
http:
  middlewares:
    domain-sentinel:
      plugin:
        domainSentinel:
          domainPathRules:
            "www3.example.com":
              sourceIPs:
                - "192.168.1.0/24"
                - "78.6.34.123"
                - "10.10.3.112"
                - "10.0.2.11"
            "www4.example.com":
              sourceIPs:
                - "0.0.0.0/0"
              pathRules:
                - path: "/admin/*"
                  sourceIPs:
                    - "10.10.4.0/24"
                    - "192.168.1.2"
                    - "80.187.117.232"
                - path: "/oai/*"
                  sourceIPs:
                    - "76.5.98.123"
```
### Explanation

- `www3.example.com` is protected globally, and all paths require IPs from the specified list.
- `www4.example.com` allows all IPs (`0.0.0.0/0`) **except** for restricted paths:
  - `/admin/*` is restricted to specific internal and external IPs.
  - `/oai/*` only allows `76.5.98.123`.

---

## Code Highlights

### `ServeHTTP` (Core Logic)

- Entry point for request handling.
- Extracts domain and path.
- Matches path rules or falls back to domain IP list.
- Validates IP → allows or blocks the request.

---

### `isPathAllowed`

- Checks if request path matches a rule using exact or wildcard matching.
- Supports `/path/*` pattern prefixing.

---

### `isIPAllowed`

- Checks if client IP matches any allowed CIDR or exact IP.
- Uses `net.ParseIP`, `net.ParseCIDR` and CIDR containment.
- Avoids misconfigured CIDRs by falling back to direct IP matching if needed.

---

## Setup instructions (Local Testing)

Step 1: **Create the Plugin**

1. Create a directory for the plugin inside the `plugins-local` folder, such as `traefik/plugins-local/src/DomainSentinel/`. The name of the plugin directory must match the name from the import statement of the `.traefik.yml` manifest file, in this case it's `DomainSentinel`.

2. Place the plugin’s Go source code files in this directory:

    - main.go (contains the plugin logic).
    - .traefik.yml (meta file for loading the plugin)
    - Other necessary Go files (if any).

    Here's an example structure:

    ```bash
    traefik/
    ├── plugins-local/
    │   └── src/
    │       └── DomainSentinel/
    │           ├── .traefik.yml
    │           ├── go.mod
    │           └── main.go
    ```

Step 2: **Configure Traefik for Local Plugins**

1. Edit your Traefik static configuration file (e.g., traefik.yml or traefik.toml), and enable experimental local plugins:

    Example: `traefik.yml`:
    ```yaml
    experimental:
      localPlugins:
        domainSentinel:
          moduleName: DomainSentinel
    ```

2. Edit your Traefik `docker-compose.yml` file and create a bind-mount for the `plugins-local` directory in order to make it available for the traefik container.

    Example: `docker-compose.yml`:
    ```yaml
    services:
      edgerouter:
        image: traefik:2.11
        container_name: traefik
        security_opt:
          - no-new-privileges:true
        ports:
          - 80:80
          - 443:443
          - 11111:11111
        restart: "always"
        volumes:
          - /etc/localtime:/etc/localtime:ro
          - /var/run/docker.sock:/var/run/docker.sock:ro
          - ./config:/etc/traefik
          - ./plugins-local:/plugins-local
          - ./letsencrypt:/letsencrypt
          - /certs:/certs
        networks:
          - edgerouter

    networks:
      edgerouter:
        external: true
    ```

Step 3: **Configure Dynamic Configuration**

1. Create a dynamic configuration file (e.g., dynamic.yml) that defines how the plugin should behave:

    Example `dynamic.yml`:
    ```yaml
    http:
      middlewares:
        domain-sentinel:
          plugin:
            domainSentinel:
              domainPathRules:
                "www3.example.com":
                  sourceIPs:
                    - "192.168.1.0/24"
                    - "78.6.34.123"
                    - "10.10.3.111"
                "www4.example.com":
                  sourceIPs:
                    - "0.0.0.0/0"
                  pathRules:
                    - path: "/admin/*"
                      sourceIPs:
                        - "10.10.4.0/24"
                        - "192.168.1.2"
                    - path: "/oai/*"
                      sourceIPs:
                        - "76.5.98.123"
                "www5.example.com":
                  sourceIPs:
                    - "10.10.3.0/24"
                    - "64.2.120.12"
    ```

    - This configuration defines the global rules for the `domain-sentinel` middleware, consisting of any combination of domain names, requested paths, and source IP addresses.

Step 4: **Associate the middleware plugin to the entrypoint**

1. Edit your Traefik static configuration file `traefik.yml`:

    Example `traefik.yml`:

    ```yaml
    entryPoints:
      webinsecure:
        address: ":80"
        http:
          middlewares:
            - domain-sentinel@file
    ```

    - This configuration ensures that the `domain-sentinel` can analyze and intervene in the whole network traffic passing through the traefik proxy.

Step 5: **Restart Traefik**

1. Start or restart traefik to load the plugin and apply the new configuration

    ```bash
    docker compose down && docker compose up -d
    ```