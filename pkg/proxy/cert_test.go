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

package proxy

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateCACert(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cert-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certPath := filepath.Join(tempDir, "ca.crt")
	keyPath := filepath.Join(tempDir, "ca.key")

	err = GenerateCACert(certPath, keyPath)
	if err != nil {
		t.Fatalf("Expected GenerateCACert to succeed, got error: %v", err)
	}

	// Verify files exist
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Errorf("Expected cert file to exist")
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Errorf("Expected key file to exist")
	}

	// Verify certificate and key can be loaded as key pair
	_, err = tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Errorf("Failed to load generated key pair: %v", err)
	}
}
