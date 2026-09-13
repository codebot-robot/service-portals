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
	"flag"
	"os"
	"testing"
	"time"
)

func TestFlags(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{
		"cmd",
		"-port", "9090",
		"-target-url", "https://github.com",
		"-cache-dir", "/tmp/git-cache",
		"-cache-ttl", "24h",
		"-cache-cleanup-interval", "5m",
		"-upstream-auth-token", "test-token",
		"-upstream-auth-header", "Authorization",
	}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	portFlag := flag.String("port", "", "Port to listen on")
	targetURLFlag := flag.String("target-url", "", "Default upstream target URL")
	cacheDirFlag := flag.String("cache-dir", "", "Directory for disk cache")
	cacheTTLFlag := flag.Duration("cache-ttl", 0, "Cache TTL duration")
	cleanupIntervalFlag := flag.Duration("cache-cleanup-interval", 0, "Cache cleanup interval")
	authTokenFlag := flag.String("upstream-auth-token", "", "Upstream auth token")
	authHeaderFlag := flag.String("upstream-auth-header", "", "Upstream auth header")
	flag.Parse()

	if *portFlag != "9090" {
		t.Errorf("expected port 9090, got %s", *portFlag)
	}
	if *targetURLFlag != "https://github.com" {
		t.Errorf("expected target URL https://github.com, got %s", *targetURLFlag)
	}
	if *cacheDirFlag != "/tmp/git-cache" {
		t.Errorf("expected cache dir /tmp/git-cache, got %s", *cacheDirFlag)
	}
	if *cacheTTLFlag != 24*time.Hour {
		t.Errorf("expected cache TTL 24h, got %v", *cacheTTLFlag)
	}
	if *cleanupIntervalFlag != 5*time.Minute {
		t.Errorf("expected cleanup interval 5m, got %v", *cleanupIntervalFlag)
	}
	if *authTokenFlag != "test-token" {
		t.Errorf("expected auth token test-token, got %s", *authTokenFlag)
	}
	if *authHeaderFlag != "Authorization" {
		t.Errorf("expected auth header Authorization, got %s", *authHeaderFlag)
	}
}
