package balancer

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewGoogleFinanceBalancer(t *testing.T) {
	t.Run("valid number of threads", func(t *testing.T) {
		threads := 3
		b, err := NewGoogleFinanceBalancer(threads)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if b == nil {
			t.Fatal("expected balancer to be non-nil")
		}
		if b.numberOfThreads != threads {
			t.Errorf("expected numberOfThreads to be %d, got %d", threads, b.numberOfThreads)
		}
		if cap(b.ch) != threads {
			t.Errorf("expected channel capacity to be %d, got %d", threads, cap(b.ch))
		}
		if len(b.ch) != threads {
			t.Errorf("expected channel length to be %d, got %d", threads, len(b.ch))
		}

		// Check if correct elements are in the channel
		expectedElements := make(map[string]bool)
		for i := 0; i < threads; i++ {
			expectedElements[elements[i]] = true
		}

		for i := 0; i < threads; i++ {
			elem := <-b.ch
			if !expectedElements[elem] {
				t.Errorf("unexpected element %q in channel", elem)
			}
			delete(expectedElements, elem)
		}
	})

	t.Run("number of threads too large", func(t *testing.T) {
		threads := len(elements) + 1
		b, err := NewGoogleFinanceBalancer(threads)
		if err == nil {
			t.Error("expected an error for too many threads, got nil")
		}
		if b != nil {
			t.Error("expected balancer to be nil on error")
		}
		expectedErr := fmt.Sprintf("NewGoogleFinanceBalancer: currently only %d threads is supported, %d given", len(elements), threads)
		if err.Error() != expectedErr {
			t.Errorf("expected error message %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("number of threads is zero", func(t *testing.T) {
		threads := 0
		b, err := NewGoogleFinanceBalancer(threads)
		if err == nil {
			t.Error("expected an error for zero threads, got nil")
		}
		if b != nil {
			t.Error("expected balancer to be nil on error")
		}
	})

	t.Run("number of threads is negative", func(t *testing.T) {
		threads := -1
		b, err := NewGoogleFinanceBalancer(threads)
		if err == nil {
			t.Error("expected an error for negative threads, got nil")
		}
		if b != nil {
			t.Error("expected balancer to be nil on error")
		}
	})
}

func TestGoogleFinanceBalancer_AcquireRelease(t *testing.T) {
	threads := 2
	b, err := NewGoogleFinanceBalancer(threads)
	if err != nil {
		t.Fatalf("failed to create balancer: %v", err)
	}

	// Acquire all elements
	e1 := b.Acquire()
	e2 := b.Acquire()

	if e1 == e2 {
		t.Errorf("acquired two identical elements: %s", e1)
	}

	// Release one element
	b.Release(e1)

	// Acquire it again
	e3 := b.Acquire()
	if e3 != e1 {
		t.Errorf("expected to acquire released element %q, but got %q", e1, e3)
	}

	// Test blocking on acquire
	var wg sync.WaitGroup
	wg.Add(1)
	acquired := false
	go func() {
		defer wg.Done()
		_ = b.Acquire() // This should block
		acquired = true
	}()

	time.Sleep(100 * time.Millisecond) // Give goroutine time to block
	if acquired {
		t.Fatal("Acquire should have blocked, but it didn't")
	}

	// Release an element to unblock the goroutine
	b.Release(e2)
	wg.Wait() // Wait for goroutine to finish
	if !acquired {
		t.Fatal("goroutine did not acquire the element after release")
	}
}
