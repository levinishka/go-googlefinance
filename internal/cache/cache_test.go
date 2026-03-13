package cache

import (
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		numCounters := int64(100)
		ttlInSec := int64(60)
		c, err := NewCache(numCounters, ttlInSec)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if c == nil {
			t.Fatal("expected cache to be non-nil")
		}
		if c.client == nil {
			t.Fatal("expected cache client to be non-nil")
		}
		expectedTTL := time.Duration(ttlInSec) * time.Second
		if c.ttl != expectedTTL {
			t.Errorf("expected ttl to be %v, got %v", expectedTTL, c.ttl)
		}
	})

	t.Run("invalid numCounters", func(t *testing.T) {
		c, err := NewCache(0, 60)
		if err == nil {
			t.Error("expected an error for zero numCounters, got nil")
		}
		if c != nil {
			t.Error("expected cache to be nil on error")
		}
	})

	t.Run("negative ttl", func(t *testing.T) {
		numCounters := int64(100)
		ttlInSec := int64(-1)
		c, err := NewCache(numCounters, ttlInSec)
		if err != nil {
			t.Fatalf("expected no error for negative ttl, got %v", err)
		}
		expectedTTL := time.Duration(ttlInSec) * time.Second
		if c.ttl != expectedTTL {
			t.Errorf("expected ttl to be %v, got %v", expectedTTL, c.ttl)
		}
	})
}

func TestCache_SetGet(t *testing.T) {
	c, err := NewCache(100, 1)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	// Ristretto is eventually consistent, so we need to wait for writes to be processed.
	defer c.client.Close()

	key := "test_key"
	value := 123.45

	t.Run("get non-existent key", func(t *testing.T) {
		_, found := c.Get(key)
		if found {
			t.Error("expected found to be false for non-existent key, but it was true")
		}
	})

	t.Run("set and get key", func(t *testing.T) {
		if !c.Set(key, value) {
			t.Error("expected Set to return true, but it returned false")
		}
		c.client.Wait() // Wait for set to be processed

		gotValue, found := c.Get(key)
		if !found {
			t.Fatal("expected found to be true for existing key, but it was false")
		}
		if gotValue != value {
			t.Errorf("expected value %f, got %f", value, gotValue)
		}
	})

	t.Run("get expired key", func(t *testing.T) {
		ttl := int64(1)
		c, _ := NewCache(100, ttl)
		defer c.client.Close()

		c.Set("expired_key", 99.9)
		c.client.Wait()

		time.Sleep(time.Duration(ttl)*time.Second + 100*time.Millisecond)

		_, found := c.Get("expired_key")
		if found {
			t.Error("expected found to be false for expired key, but it was true")
		}
	})
}

func TestCache_Clear(t *testing.T) {
	c, err := NewCache(100, 60)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.client.Close()

	c.Set("key1", 1.0)
	c.Clear()
	c.client.Wait()

	_, found := c.Get("key1")
	if found {
		t.Error("expected to not find key after Clear, but it was found")
	}
}
