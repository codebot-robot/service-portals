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
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gke-labs/service-portals/pkg/proxy"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	caCertPath := os.Getenv("CA_CERT_PATH")
	caKeyPath := os.Getenv("CA_KEY_PATH")

	// TargetURL is nil since this is a forward proxy
	p, err := proxy.NewHTTPProxy(nil, "", "", caCertPath, caKeyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create proxy: %v\n", err)
		os.Exit(1)
	}

	cacheDir := os.Getenv("CACHE_DIR")
	if cacheDir == "" {
		cacheDir = "/tmp/artifact-cache"
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create cache dir: %v\n", err)
		os.Exit(1)
	}

	diskCache := diskcache.New(cacheDir)
	cacheTransport := httpcache.NewTransport(diskCache)
	p.Transport = cacheTransport

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: p,
	}

	errChan := make(chan error, 1)
	go func() {
		log.Printf("Starting artifact-portal caching proxy on :%s (cache: %s)", port, cacheDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("server failed: %w", err)
		}
	}()

	select {
	case err := <-errChan:
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	case <-ctx.Done():
		log.Println("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "server shutdown failed: %v\n", err)
			os.Exit(1)
		}
	}
}
