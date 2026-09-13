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
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gke-labs/service-portals/pkg/cache"
	"github.com/gke-labs/service-portals/pkg/gitproxy"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("gitproxy-server", flag.ContinueOnError)

	portFlag := flags.String("port", "", "Port to listen on (defaults to PORT env var or 8080)")
	targetURLFlag := flags.String("target-url", "", "Default upstream target URL (defaults to TARGET_URL env var)")
	cacheDirFlag := flags.String("cache-dir", "", "Directory for disk cache (defaults to CACHE_DIR env var, in-memory if empty)")
	cacheTTLFlag := flags.Duration("cache-ttl", 0, "Cache TTL duration (defaults to CACHE_TTL env var or 168h)")
	cleanupIntervalFlag := flags.Duration("cache-cleanup-interval", 0, "Cache cleanup interval (defaults to CACHE_CLEANUP_INTERVAL env var or 10m)")
	authTokenFlag := flags.String("upstream-auth-token", "", "Upstream authentication token (defaults to UPSTREAM_AUTH_TOKEN env var)")
	authHeaderFlag := flags.String("upstream-auth-header", "", "Upstream authentication header name (defaults to UPSTREAM_AUTH_HEADER env var or Authorization)")

	if err := flags.Parse(args); err != nil {
		return err
	}

	port := *portFlag
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "8080"
	}

	targetURL := *targetURLFlag
	if targetURL == "" {
		targetURL = os.Getenv("TARGET_URL")
	}

	cacheDir := *cacheDirFlag
	if cacheDir == "" {
		cacheDir = os.Getenv("CACHE_DIR")
	}

	cacheTTL := *cacheTTLFlag
	if cacheTTL <= 0 {
		if ttlEnv := os.Getenv("CACHE_TTL"); ttlEnv != "" {
			if d, err := time.ParseDuration(ttlEnv); err == nil {
				cacheTTL = d
			}
		}
	}
	if cacheTTL <= 0 {
		cacheTTL = 7 * 24 * time.Hour
	}

	cleanupInterval := *cleanupIntervalFlag
	if cleanupInterval <= 0 {
		if intervalEnv := os.Getenv("CACHE_CLEANUP_INTERVAL"); intervalEnv != "" {
			if d, err := time.ParseDuration(intervalEnv); err == nil {
				cleanupInterval = d
			}
		}
	}
	if cleanupInterval <= 0 {
		cleanupInterval = 10 * time.Minute
	}

	authToken := *authTokenFlag
	if authToken == "" {
		authToken = os.Getenv("UPSTREAM_AUTH_TOKEN")
	}

	authHeader := *authHeaderFlag
	if authHeader == "" {
		authHeader = os.Getenv("UPSTREAM_AUTH_HEADER")
	}
	if authHeader == "" {
		authHeader = "Authorization"
	}

	var c cache.Cache
	if cacheDir != "" {
		diskCache, err := cache.NewDiskCache(cacheDir, cacheTTL, cleanupInterval)
		if err != nil {
			return fmt.Errorf("failed to initialize disk cache in %s: %w", cacheDir, err)
		}
		c = diskCache
		log.Printf("Using disk cache in %s with TTL %v", cacheDir, cacheTTL)
	} else {
		c = cache.NewInMemoryCache(cleanupInterval)
		log.Printf("Using in-memory cache with TTL %v", cacheTTL)
	}

	config := gitproxy.Config{
		DefaultTargetURL:  targetURL,
		DefaultAuthToken:  authToken,
		DefaultAuthHeader: authHeader,
		Cache:             c,
		CacheTTL:          cacheTTL,
	}

	return gitproxy.Run(ctx, config, port)
}
