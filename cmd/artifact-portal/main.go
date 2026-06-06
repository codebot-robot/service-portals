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
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gke-labs/service-portals/pkg/portals"
	"github.com/gke-labs/service-portals/pkg/proxy"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Set defaults for artifact-portal if they are not explicitly configured.
	caCertPath := os.Getenv("CA_CERT_PATH")
	caKeyPath := os.Getenv("CA_KEY_PATH")
	if caCertPath == "" {
		caCertPath = "./certs/ca.crt"
		os.Setenv("CA_CERT_PATH", caCertPath)
	}
	if caKeyPath == "" {
		caKeyPath = "./certs/ca.key"
		os.Setenv("CA_KEY_PATH", caKeyPath)
	}

	// Generate a self-signed Root CA automatically if it doesn't exist.
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		log.Printf("CA certificate not found at %s. Generating self-signed root CA...", caCertPath)
		if err := proxy.GenerateCACert(caCertPath, caKeyPath); err != nil {
			log.Fatalf("Failed to generate CA certificate: %v", err)
		}
		log.Printf("Successfully generated root CA certificate at %s and key at %s", caCertPath, caKeyPath)
	}

	if os.Getenv("CACHE_DIR") == "" {
		os.Setenv("CACHE_DIR", "./cache")
		log.Printf("No CACHE_DIR set. Defaulting to disk cache at ./cache")
	}

	if os.Getenv("CACHE_TTL") == "" {
		os.Setenv("CACHE_TTL", "24h")
		log.Printf("No CACHE_TTL set. Defaulting cache TTL to 24 hours")
	}

	config := portals.Config{
		DefaultTargetURL: "https://pypi.org",
		CacheTTL:         24 * time.Hour,
	}

	log.Println("Starting Artifact Caching Proxy...")
	if err := portals.Run(ctx, config); err != nil {
		fmt.Fprintf(os.Stderr, "Artifact Caching Proxy error: %v\n", err)
		os.Exit(1)
	}
}
