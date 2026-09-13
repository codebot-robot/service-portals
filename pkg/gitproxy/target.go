// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitproxy

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// ResolveUpstreamURL resolves the target upstream URL from an incoming HTTP request.
func ResolveUpstreamURL(r *http.Request, defaultTarget string) (*url.URL, error) {
	// 1. Direct proxy request with full URL
	if r.URL.Scheme != "" && r.URL.Host != "" {
		return r.URL, nil
	}

	rawPath := r.URL.EscapedPath()
	if rawPath == "" {
		rawPath = r.URL.Path
	}

	trimmed := rawPath
	if strings.HasPrefix(trimmed, "/") {
		trimmed = trimmed[1:]
	}

	// 2. Path starting with scheme prefix: /https://, /https:/, /http://, /http:/, /https/, /http/
	for _, scheme := range []string{"https", "http"} {
		prefixDbl := scheme + "://"
		prefixSgl := scheme + ":/"
		prefixSlash := scheme + "/"

		if strings.HasPrefix(trimmed, prefixDbl) {
			rest := trimmed[len(prefixDbl):]
			return parseHostAndPath(scheme, rest, r.URL.RawQuery)
		}
		if strings.HasPrefix(trimmed, prefixSgl) {
			rest := trimmed[len(prefixSgl):]
			return parseHostAndPath(scheme, rest, r.URL.RawQuery)
		}
		if strings.HasPrefix(trimmed, prefixSlash) {
			rest := trimmed[len(prefixSlash):]
			return parseHostAndPath(scheme, rest, r.URL.RawQuery)
		}
	}

	// 3. Path starting with a hostname (e.g. /github.com/org/repo.git or /127.0.0.1:8080/repo.git)
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) > 0 && isLikelyHostname(parts[0]) {
		host := parts[0]
		subPath := ""
		if len(parts) > 1 {
			subPath = "/" + parts[1]
		}
		scheme := "https"
		if isLocalHost(host) {
			scheme = "http"
		}
		if defaultTarget != "" {
			if defU, err := url.Parse(defaultTarget); err == nil && defU.Scheme != "" {
				scheme = defU.Scheme
			}
		}
		return buildURL(scheme, host, subPath, r.URL.RawQuery)
	}

	// 4. Relative path with configured defaultTarget
	if defaultTarget != "" {
		base, err := url.Parse(defaultTarget)
		if err != nil {
			return nil, fmt.Errorf("invalid default target URL %q: %w", defaultTarget, err)
		}
		relPath := r.URL.Path
		if !strings.HasPrefix(relPath, "/") {
			relPath = "/" + relPath
		}
		joinedPath := strings.TrimRight(base.Path, "/") + relPath
		return buildURL(base.Scheme, base.Host, joinedPath, r.URL.RawQuery)
	}

	// 5. Check r.Host if not local and not dummy example.com
	if r.Host != "" && !isLocalHost(r.Host) && r.Host != "example.com" {
		return buildURL("https", r.Host, r.URL.Path, r.URL.RawQuery)
	}

	return nil, fmt.Errorf("unable to determine upstream target URL for path %q", r.URL.Path)
}

func parseHostAndPath(scheme, rest, rawQuery string) (*url.URL, error) {
	parts := strings.SplitN(rest, "/", 2)
	host := parts[0]
	subPath := ""
	if len(parts) > 1 {
		subPath = "/" + parts[1]
	}
	return buildURL(scheme, host, subPath, rawQuery)
}

func buildURL(scheme, host, path, rawQuery string) (*url.URL, error) {
	if scheme == "" {
		scheme = "https"
	}
	u := &url.URL{
		Scheme:   scheme,
		Host:     host,
		Path:     path,
		RawQuery: rawQuery,
	}
	return u, nil
}

func isLikelyHostname(s string) bool {
	if s == "" {
		return false
	}
	if strings.HasSuffix(s, ".git") {
		return false
	}
	if h, _, err := net.SplitHostPort(s); err == nil {
		s = h
	}
	if net.ParseIP(s) != nil {
		return true
	}
	if s == "localhost" {
		return true
	}
	if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		tld := parts[len(parts)-1]
		if len(tld) >= 2 && !strings.EqualFold(tld, "git") {
			return true
		}
	}
	return false
}

func isLocalHost(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err == nil {
		host = h
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == ""
}
