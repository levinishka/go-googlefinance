package cache

import (
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		maxSize := int64(100)
		ttlInSec := int64(60)
		c, err := NewCache[float64](maxSize, ttlInSec)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if c == nil {
			t.Fatal("expected cache to be non-nil")
		}
		expectedTTL := time.Duration(ttlInSec) * time.Second
		if c.ttl != expectedTTL {
			t.Errorf("expected ttl to be %v, got %v", expectedTTL, c.ttl)
		}
	})

	t.Run("invalid maxSize", func(t *testing.T) {
		c, err := NewCache[float64](0, 60)
		if err == nil {
			t.Error("expected an error for zero maxSize, got nil")
		}
		if c != nil {
			t.Error("expected cache to be nil on error")
		}
	})

	t.Run("negative ttl", func(t *testing.T) {
		maxSize := int64(100)
		ttlInSec := int64(-1)
		c, err := NewCache[float64](maxSize, ttlInSec)
		if err != nil {
			t.Fatalf("expected no error for negative ttl, got %v", err)
		}
		expectedTTL := time.Duration(ttlInSec) * time.Second
		if c.ttl != expectedTTL {
			t.Errorf("expected ttl to be %v, got %v", expectedTTL, c.ttl)
		}
		if c.Set("k", 1.0) {
			t.Error("expected Set to return false when ttl <= 0")
		}
	})
}

func TestCache_SetGet(t *testing.T) {
	c, err := NewCache[float64](100, 1)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

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
		ec, _ := NewCache[float64](100, ttl)

		ec.Set("expired_key", 99.9)
		time.Sleep(time.Duration(ttl)*time.Second + 100*time.Millisecond)

		_, found := ec.Get("expired_key")
		if found {
			t.Error("expected found to be false for expired key, but it was true")
		}
	})

	t.Run("set and get struct value", func(t *testing.T) {
		type asset struct {
			Price    float64
			Name     string
			Currency string
		}
		sc, err := NewCache[asset](100, 60)
		if err != nil {
			t.Fatalf("failed to create struct cache: %v", err)
		}

		want := asset{Price: 250.5, Name: "Vanguard Total Stock Market ETF", Currency: "USD"}
		if !sc.Set("VTI", want) {
			t.Fatal("expected Set to return true")
		}

		got, found := sc.Get("VTI")
		if !found {
			t.Fatal("expected found to be true for existing struct key")
		}
		if got != want {
			t.Errorf("expected %+v, got %+v", want, got)
		}
	})

	t.Run("respects maxSize capacity", func(t *testing.T) {
		bc, err := NewCache[float64](2, 60)
		if err != nil {
			t.Fatalf("failed to create bounded cache: %v", err)
		}

		bc.Set("k1", 1.0)
		bc.Set("k2", 2.0)
		bc.Set("k3", 3.0)

		bc.mu.RLock()
		count := len(bc.entries)
		bc.mu.RUnlock()

		if count > 2 {
			t.Errorf("expected at most 2 entries, got %d", count)
		}
	})
}

func TestCache_Clear(t *testing.T) {
	c, err := NewCache[float64](100, 60)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	c.Set("key1", 1.0)
	c.Clear()

	_, found := c.Get("key1")
	if found {
		t.Error("expected to not find key after Clear, but it was found")
	}
}
