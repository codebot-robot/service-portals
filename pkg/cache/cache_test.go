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
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func TestInMemoryCache(t *testing.T) {
	c := NewInMemoryCache(0)

	// Test Set and Get
	c.Set("key1", []byte("value1"), 1*time.Minute)
	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("Expected to find key1")
	}
	if !bytes.Equal(val, []byte("value1")) {
		t.Errorf("Expected value1, got %s", val)
	}

	// Test Delete
	c.Delete("key1")
	_, ok = c.Get("key1")
	if ok {
		t.Fatal("Expected key1 to be deleted")
	}

	// Test Expiration
	c.Set("key2", []byte("value2"), 1*time.Millisecond)
	time.Sleep(2 * time.Millisecond)
	_, ok = c.Get("key2")
	if ok {
		t.Fatal("Expected key2 to be expired")
	}
}

func TestDiskCache(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := NewDiskCache(tmpDir, 50*time.Millisecond, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	// Test Set and Get
	c.Set("disk_key1", []byte("disk_value1"), 1*time.Minute)
	val, ok := c.Get("disk_key1")
	if !ok {
		t.Fatal("Expected to find disk_key1")
	}
	if !bytes.Equal(val, []byte("disk_value1")) {
		t.Errorf("Expected disk_value1, got %s", val)
	}

	// Test Open (io.ReadSeekCloser)
	r, err := c.Open("disk_key1")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	buf := make([]byte, 4)
	n, err := r.Read(buf)
	if err != nil || n != 4 || string(buf) != "disk" {
		t.Errorf("expected read 'disk', got %s (err %v)", string(buf), err)
	}
	// Test Seek
	if _, err := r.Seek(0, 0); err != nil {
		t.Fatalf("Seek failed: %v", err)
	}
	all, err := io.ReadAll(r)
	if err != nil || string(all) != "disk_value1" {
		t.Errorf("expected disk_value1 after seek, got %s", string(all))
	}
	r.Close()

	// Test Put stream
	if err := c.Put("stream_key", strings.NewReader("stream_value")); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	sVal, ok := c.Get("stream_key")
	if !ok || string(sVal) != "stream_value" {
		t.Errorf("expected stream_value, got %s", string(sVal))
	}

	// Test Delete
	c.Delete("disk_key1")
	_, ok = c.Get("disk_key1")
	if ok {
		t.Fatal("Expected disk_key1 to be deleted")
	}

	// Test Expiration
	c.Set("disk_key2", []byte("disk_value2"), 20*time.Millisecond)
	time.Sleep(70 * time.Millisecond)
	_, ok = c.Get("disk_key2")
	if ok {
		t.Fatal("Expected disk_key2 to be expired")
	}
}
