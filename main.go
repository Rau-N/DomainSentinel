package DomainSentinel

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "strings"
)

// Config holds the plugin configuration.
type Config struct {
    DomainPathRules map[string]DomainConfig `json:"domainPathRules,omitempty"`
}

// DomainConfig holds domain-wide source IPs and path-specific configurations.
type DomainConfig struct {
    SourceIPs []string     `json:"sourceIPs,omitempty"` // Domain-wide source IPs
    PathRules []PathConfig `json:"pathRules,omitempty"` // Path-specific rules
}

// PathConfig holds the path and source IPs for a specific path under a domain.
type PathConfig struct {
    Path      string   `json:"path,omitempty"`
    SourceIPs []string `json:"sourceIPs,omitempty"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
    return &Config{
        DomainPathRules: make(map[string]DomainConfig),
    }
}

// DomainSentinel middleware struct
type DomainSentinel struct {
    next   http.Handler
    config *Config
    name   string
}

// New creates a new DomainSentinel middleware.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
    return &DomainSentinel{
        next:   next,
        config: config,
        name:   name,
    }, nil
}

func (ds *DomainSentinel) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
    fmt.Println("Plugin: DomainSentinel")
    host := req.Host
    var requestedDomain string

    // Handle host with or without port
    if strings.Contains(host, ":") {
        var err error
        requestedDomain, _, err = net.SplitHostPort(host)
        if err != nil {
            requestedDomain = host
        }
    } else {
        requestedDomain = host
    }

    fmt.Println("Requested Domain:", requestedDomain)

    // Allow request if domain is not found in the configuration. 
    domainConfig, domainExists := ds.config.DomainPathRules[requestedDomain]
    if !domainExists {
        fmt.Println("No config found for domain:", requestedDomain)
        ds.next.ServeHTTP(rw, req)
        return
    }

    fmt.Println("SourceIPs: ", domainConfig.SourceIPs)
    fmt.Println("Requested Path: ", req.URL.Path)

    // Check the path-specific rules first
    for _, pathRule := range domainConfig.PathRules {
        fmt.Println("Configured Path: ", pathRule.Path)
        if isPathAllowed(req.URL.Path, pathRule.Path) {
            fmt.Println("Path matches")
            fmt.Println("SourceIPs: ", pathRule.SourceIPs)
            if !ds.isIPAllowed(req, pathRule.SourceIPs) {
                http.Error(rw, "DS: Forbidden", http.StatusForbidden)
                return
            }
            ds.next.ServeHTTP(rw, req)
            return
        }
    }

    // If no path-specific rules matched, check the domain-wide rules
    if !ds.isIPAllowed(req, domainConfig.SourceIPs) {
        http.Error(rw, "DS: Forbidden", http.StatusForbidden)
        return
    }

    ds.next.ServeHTTP(rw, req)
}

// isPathAllowed checks if the request path matches any allowed path patterns.
func isPathAllowed(reqPath string, pathPattern string) bool {
    if strings.HasSuffix(pathPattern, "/*") {
        basePath := strings.TrimSuffix(pathPattern, "/*")
        if strings.HasPrefix(reqPath, basePath) {
            return true
        }
    } else if reqPath == pathPattern {
        return true
    }
    return false
}

func (ds *DomainSentinel) isIPAllowed(req *http.Request, allowedIPs []string) bool {
    ip, _, err := net.SplitHostPort(req.RemoteAddr)
    if err != nil {
        fmt.Println("Error splitting host and port: ", err)
        return false
    }

    allowedIPsString := fmt.Sprint(allowedIPs)
    cleanedAllowedIPs := cleanCIDR(allowedIPsString)
    cleanedAllowedIPsArray := strings.Split(strings.Trim(cleanedAllowedIPs, "[]"), " ")

    fmt.Println("Cleaned source IP list: ", cleanedAllowedIPsArray)

    for _, cidr := range cleanedAllowedIPsArray {
        _, ipNet, err := net.ParseCIDR(cidr)
        if err != nil {
            if cidr == ip {
                fmt.Println("Direct IP match found:", ip)
                return true
            }
            continue
        }

        if ipNet.Contains(net.ParseIP(ip)) {
            fmt.Println("IP match found in CIDR:", cidr)
            return true
        }
    }

    fmt.Println("No IP match found, denying access")
    return false
}

// cleanCIDR replaces "║24║" with an empty string and remaining "║" with a space.
func cleanCIDR(cidr string) string {
    cleaned := strings.ReplaceAll(cidr, "║24║", "")
    cleaned = strings.ReplaceAll(cleaned, "║", " ")
    return cleaned
}
