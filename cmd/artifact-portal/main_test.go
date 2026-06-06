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

package main

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gke-labs/service-portals/pkg/cache"
	"github.com/gke-labs/service-portals/pkg/proxy"
)

func TestArtifactPortalCaching(t *testing.T) {
	// 1. Create a temp directory for disk cache
	tempDir, err := os.MkdirTemp("", "artifact-portal-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	diskCache, err := cache.NewDiskCache(tempDir)
	if err != nil {
		t.Fatalf("Failed to create disk cache: %v", err)
	}

	// 2. Set up upstream server
	callCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock-wheel-content"))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("Failed to parse upstream URL: %v", err)
	}

	// 3. Create proxy
	p, err := proxy.NewHTTPProxy(upstreamURL, "", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Use our DiskCache inside a CachingTransport
	p.Transport = proxy.NewCachingTransport(diskCache, http.DefaultTransport, 1*time.Minute)

	// 4. Send first request to the proxy
	req := httptest.NewRequest("GET", upstreamURL.String(), nil)
	w1 := httptest.NewRecorder()
	p.ServeHTTP(w1, req)

	resp1 := w1.Result()
	if resp1.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp1.Status)
	}
	body1, _ := io.ReadAll(resp1.Body)
	if string(body1) != "mock-wheel-content" {
		t.Errorf("Expected body 'mock-wheel-content', got '%s'", string(body1))
	}
	if callCount != 1 {
		t.Errorf("Expected upstream to be called exactly 1 time, got %d", callCount)
	}

	// 5. Send second request (should be served from disk cache)
	w2 := httptest.NewRecorder()
	p.ServeHTTP(w2, req)

	resp2 := w2.Result()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp2.Status)
	}
	body2, _ := io.ReadAll(resp2.Body)
	if string(body2) != "mock-wheel-content" {
		t.Errorf("Expected body 'mock-wheel-content', got '%s'", string(body2))
	}
	if callCount != 1 {
		t.Errorf("Expected upstream to NOT be called a second time (cached), got %d", callCount)
	}
}

func TestArtifactPortalMITMCaching(t *testing.T) {
	// Create temporary directory for certificates and cache
	tempDir, err := os.MkdirTemp("", "artifact-portal-mitm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certPath := filepath.Join(tempDir, "ca.crt")
	keyPath := filepath.Join(tempDir, "ca.key")

	err = proxy.GenerateCACert(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	diskCache, err := cache.NewDiskCache(filepath.Join(tempDir, "cache"))
	if err != nil {
		t.Fatalf("Failed to create disk cache: %v", err)
	}

	// Create proxy with MITM capabilities
	targetURL, _ := url.Parse("https://pypi.org")
	p, err := proxy.NewHTTPProxy(targetURL, "", "", certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}
	p.Transport = proxy.NewCachingTransport(diskCache, http.DefaultTransport, 1*time.Minute)

	// Since full HTTPS dynamic MITM test would require setting up a custom dialer/server tunnel,
	// let's verify that p.GetCertificate is capable of signing certs for arbitrary hosts.
	hello := &tls.ClientHelloInfo{
		ServerName: "files.pythonhosted.org",
	}
	cert, err := p.GetCertificate(hello)
	if err != nil {
		t.Fatalf("Failed to get dynamically signed certificate: %v", err)
	}
	if cert == nil {
		t.Fatalf("Expected a non-nil certificate")
	}
}
