package balancer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestColumnName(t *testing.T) {
	cases := []struct {
		index int
		want  string
	}{
		{0, "A"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{51, "AZ"},
		{52, "BA"},
	}
	for _, tc := range cases {
		if got := columnName(tc.index); got != tc.want {
			t.Errorf("columnName(%d) = %q, want %q", tc.index, got, tc.want)
		}
	}
}

func TestNewGoogleFinanceBalancer(t *testing.T) {
	t.Run("valid number of threads and columnsPerSlot", func(t *testing.T) {
		threads := 3
		columnsPerSlot := 3
		b, err := NewGoogleFinanceBalancer(threads, columnsPerSlot)
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

		expectedSlots := []ColumnSlot{
			{Start: "A", End: "C"},
			{Start: "D", End: "F"},
			{Start: "G", End: "I"},
		}
		for i, want := range expectedSlots {
			got := <-b.ch
			if got != want {
				t.Errorf("slot %d = %+v, want %+v", i, got, want)
			}
		}
	})

	t.Run("slots spanning past Z", func(t *testing.T) {
		threads := 10
		columnsPerSlot := 3
		b, err := NewGoogleFinanceBalancer(threads, columnsPerSlot)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		var last ColumnSlot
		for i := 0; i < threads; i++ {
			last = <-b.ch
		}
		want := ColumnSlot{Start: "AB", End: "AD"}
		if last != want {
			t.Errorf("last slot = %+v, want %+v", last, want)
		}
	})

	t.Run("number of threads is zero", func(t *testing.T) {
		b, err := NewGoogleFinanceBalancer(0, 3)
		if err == nil {
			t.Error("expected an error for zero threads, got nil")
		}
		if b != nil {
			t.Error("expected balancer to be nil on error")
		}
	})

	t.Run("number of threads is negative", func(t *testing.T) {
		b, err := NewGoogleFinanceBalancer(-1, 3)
		if err == nil {
			t.Error("expected an error for negative threads, got nil")
		}
		if b != nil {
			t.Error("expected balancer to be nil on error")
		}
	})

	t.Run("columnsPerSlot is zero or negative", func(t *testing.T) {
		for _, cols := range []int{0, -2} {
			b, err := NewGoogleFinanceBalancer(2, cols)
			if err == nil {
				t.Errorf("expected an error for columnsPerSlot=%d, got nil", cols)
			}
			if b != nil {
				t.Errorf("expected balancer to be nil on error for columnsPerSlot=%d", cols)
			}
		}
	})
}

func TestGoogleFinanceBalancer_AcquireRelease(t *testing.T) {
	threads := 2
	b, err := NewGoogleFinanceBalancer(threads, 3)
	if err != nil {
		t.Fatalf("failed to create balancer: %v", err)
	}

	ctx := context.Background()

	// Acquire all elements
	e1, err := b.Acquire(ctx)
	if err != nil {
		t.Fatalf("unexpected error acquiring e1: %v", err)
	}
	e2, err := b.Acquire(ctx)
	if err != nil {
		t.Fatalf("unexpected error acquiring e2: %v", err)
	}

	if e1 == e2 {
		t.Errorf("acquired two identical slots: %+v", e1)
	}

	// Release one element
	b.Release(e1)

	// Acquire it again
	e3, err := b.Acquire(ctx)
	if err != nil {
		t.Fatalf("unexpected error acquiring e3: %v", err)
	}
	if e3 != e1 {
		t.Errorf("expected to acquire released slot %+v, but got %+v", e1, e3)
	}

	// Test context cancellation when all slots are busy
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.Acquire(cancelCtx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	// Test blocking on acquire
	var wg sync.WaitGroup
	wg.Add(1)
	acquired := false
	go func() {
		defer wg.Done()
		if _, err := b.Acquire(ctx); err != nil { // This should block
			t.Errorf("unexpected error in blocking acquire: %v", err)
			return
		}
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
		t.Fatal("goroutine did not acquire the slot after release")
	}
}
