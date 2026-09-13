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
	"testing"
)

func TestClassifyGitRequest(t *testing.T) {
	tests := []struct {
		name              string
		method            string
		path              string
		body              []byte
		expectedType      RequestType
		expectedCacheable bool
	}{
		{
			name:              "Smart HTTP refs discovery",
			method:            http.MethodGet,
			path:              "/org/repo.git/info/refs?service=git-upload-pack",
			expectedType:      RequestTypeRefs,
			expectedCacheable: false,
		},
		{
			name:              "Dumb HTTP refs discovery",
			method:            http.MethodGet,
			path:              "/org/repo.git/info/refs",
			expectedType:      RequestTypeRefs,
			expectedCacheable: false,
		},
		{
			name:              "HEAD ref request",
			method:            http.MethodGet,
			path:              "/org/repo.git/HEAD",
			expectedType:      RequestTypeRefs,
			expectedCacheable: false,
		},
		{
			name:              "Protocol v2 ls-refs",
			method:            http.MethodPost,
			path:              "/org/repo.git/git-upload-pack",
			body:              []byte("0014command=ls-refs\n0014agent=git/2.47.0\n00010009peel\n000csymrefs\n0000"),
			expectedType:      RequestTypeUploadPackLsRefs,
			expectedCacheable: false,
		},
		{
			name:              "Protocol v2 fetch",
			method:            http.MethodPost,
			path:              "/org/repo.git/git-upload-pack",
			body:              []byte("0011command=fetch\n0014agent=git/2.47.0\n0001000dthin-pack\n0032want 7f83b1657ff1fc5354dc1008ecf50686d3c52b78\n0009done\n0000"),
			expectedType:      RequestTypeUploadPackFetch,
			expectedCacheable: true,
		},
		{
			name:              "Protocol v0/v1 fetch",
			method:            http.MethodPost,
			path:              "/org/repo.git/git-upload-pack",
			body:              []byte("0032want 7f83b1657ff1fc5354dc1008ecf50686d3c52b78 multi_ack_detailed side-band-64k\n00000009done\n"),
			expectedType:      RequestTypeUploadPackFetch,
			expectedCacheable: true,
		},
		{
			name:              "Push ref discovery",
			method:            http.MethodGet,
			path:              "/org/repo.git/info/refs?service=git-receive-pack",
			expectedType:      RequestTypeReceivePack,
			expectedCacheable: false,
		},
		{
			name:              "Push upload-pack",
			method:            http.MethodPost,
			path:              "/org/repo.git/git-receive-pack",
			body:              []byte("003c0000000000000000000000000000000000000000 7f83b165 refs/heads/main\n"),
			expectedType:      RequestTypeReceivePack,
			expectedCacheable: false,
		},
		{
			name:              "Loose object SHA1",
			method:            http.MethodGet,
			path:              "/org/repo.git/objects/4b/825dc642cb6eb9a060e54bf8d69288fbee4904",
			expectedType:      RequestTypeObject,
			expectedCacheable: true,
		},
		{
			name:              "Packfile",
			method:            http.MethodGet,
			path:              "/org/repo.git/objects/pack/pack-1234567890abcdef1234567890abcdef12345678.pack",
			expectedType:      RequestTypeObject,
			expectedCacheable: true,
		},
		{
			name:              "Pack index",
			method:            http.MethodGet,
			path:              "/org/repo.git/objects/pack/pack-1234567890abcdef1234567890abcdef12345678.idx",
			expectedType:      RequestTypeObject,
			expectedCacheable: true,
		},
		{
			name:              "LFS object",
			method:            http.MethodGet,
			path:              "/org/repo.git/info/lfs/objects/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			expectedType:      RequestTypeObject,
			expectedCacheable: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			actualType := ClassifyGitRequest(req, tc.body)
			if actualType != tc.expectedType {
				t.Errorf("expected RequestType %v, got %v", tc.expectedType, actualType)
			}
			if actualType.IsCacheable() != tc.expectedCacheable {
				t.Errorf("expected IsCacheable %v, got %v", tc.expectedCacheable, actualType.IsCacheable())
			}
		})
	}
}
