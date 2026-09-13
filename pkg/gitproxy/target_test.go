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
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestResolveUpstreamURL(t *testing.T) {
	tests := []struct {
		name          string
		requestURL    string
		host          string
		defaultTarget string
		expectedURL   string
		expectError   bool
	}{
		{
			name:        "Path with https:// prefix",
			requestURL:  "/https://github.com/org/repo.git/info/refs?service=git-upload-pack",
			expectedURL: "https://github.com/org/repo.git/info/refs?service=git-upload-pack",
		},
		{
			name:        "Path with https:/ prefix",
			requestURL:  "/https:/github.com/org/repo.git/info/refs?service=git-upload-pack",
			expectedURL: "https://github.com/org/repo.git/info/refs?service=git-upload-pack",
		},
		{
			name:        "Path with https/ prefix",
			requestURL:  "/https/github.com/org/repo.git/info/refs?service=git-upload-pack",
			expectedURL: "https://github.com/org/repo.git/info/refs?service=git-upload-pack",
		},
		{
			name:        "Path with http:// prefix",
			requestURL:  "/http://git.internal:8080/org/repo.git/objects/pack/pack-123.pack",
			expectedURL: "http://git.internal:8080/org/repo.git/objects/pack/pack-123.pack",
		},
		{
			name:        "Path starting with domain name",
			requestURL:  "/github.com/org/repo.git/git-upload-pack",
			expectedURL: "https://github.com/org/repo.git/git-upload-pack",
		},
		{
			name:          "Relative path with default target",
			requestURL:    "/org/repo.git/info/refs?service=git-upload-pack",
			defaultTarget: "https://github.com",
			expectedURL:   "https://github.com/org/repo.git/info/refs?service=git-upload-pack",
		},
		{
			name:          "Relative path with default target containing subpath",
			requestURL:    "/repo.git/info/refs",
			defaultTarget: "https://github.com/base-org",
			expectedURL:   "https://github.com/base-org/repo.git/info/refs",
		},
		{
			name:        "External host header",
			requestURL:  "/org/repo.git/info/refs",
			host:        "gitlab.com",
			expectedURL: "https://gitlab.com/org/repo.git/info/refs",
		},
		{
			name:        "Direct proxy request with full URL",
			requestURL:  "https://github.com/org/repo.git/info/refs",
			expectedURL: "https://github.com/org/repo.git/info/refs",
		},
		{
			name:        "Unknown relative path without default target",
			requestURL:  "/unknown/path",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if u, err := url.Parse(tc.requestURL); err == nil && u.Scheme != "" && u.Host != "" {
				req = httptest.NewRequest("GET", tc.requestURL, nil)
				req.URL = u
			} else {
				req = httptest.NewRequest("GET", tc.requestURL, nil)
			}
			req.Host = tc.host

			resolved, err := ResolveUpstreamURL(req, tc.defaultTarget)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error, got resolved URL %v", resolved)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resolved.String() != tc.expectedURL {
				t.Errorf("expected %s, got %s", tc.expectedURL, resolved.String())
			}
		})
	}
}
