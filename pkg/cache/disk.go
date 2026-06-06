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

package cache

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type diskItem struct {
	Value      []byte
	Expiration time.Time
}

// DiskCache is a file-system backed implementation of Cache.
type DiskCache struct {
	mu  sync.RWMutex
	dir string
}

// NewDiskCache creates a new DiskCache in the specified directory.
func NewDiskCache(dir string) (*DiskCache, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	return &DiskCache{dir: dir}, nil
}

func (d *DiskCache) keyPath(key string) string {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	hash := hex.EncodeToString(hasher.Sum(nil))
	return filepath.Join(d.dir, hash)
}

// Get retrieves an item from the disk cache.
func (d *DiskCache) Get(key string) ([]byte, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	path := d.keyPath(key)
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer file.Close()

	var item diskItem
	dec := gob.NewDecoder(file)
	if err := dec.Decode(&item); err != nil {
		return nil, false
	}

	if time.Now().After(item.Expiration) {
		return nil, false
	}

	return item.Value, true
}

// Set adds an item to the disk cache with a TTL.
func (d *DiskCache) Set(key string, value []byte, ttl time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	path := d.keyPath(key)
	file, err := os.Create(path)
	if err != nil {
		return
	}
	defer file.Close()

	item := diskItem{
		Value:      value,
		Expiration: time.Now().Add(ttl),
	}

	enc := gob.NewEncoder(file)
	_ = enc.Encode(item)
}

// Delete removes an item from the disk cache.
func (d *DiskCache) Delete(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	path := d.keyPath(key)
	_ = os.Remove(path)
}

func init() {
	gob.Register(diskItem{})
}
