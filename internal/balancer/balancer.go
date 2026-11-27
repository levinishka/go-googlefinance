package balancer

import (
	"context"
	"fmt"
)

type ColumnSlot struct {
	Start string
	End   string
}

type GoogleFinanceBalancer struct {
	numberOfThreads int

	ch chan ColumnSlot
}

func columnName(index int) string {
	name := ""
	for index >= 0 {
		name = string(rune('A'+(index%26))) + name
		index = index/26 - 1
	}
	return name
}

func NewGoogleFinanceBalancer(numberOfThreads int, columnsPerSlot int) (*GoogleFinanceBalancer, error) {
	if numberOfThreads <= 0 {
		return nil, fmt.Errorf("NewGoogleFinanceBalancer: numberOfThreads should be > 0, %d given", numberOfThreads)
	}

	if columnsPerSlot <= 0 {
		return nil, fmt.Errorf("NewGoogleFinanceBalancer: columnsPerSlot should be > 0, %d given", columnsPerSlot)
	}

	balancer := &GoogleFinanceBalancer{
		numberOfThreads: numberOfThreads,
		ch:              make(chan ColumnSlot, numberOfThreads),
	}

	for i := 0; i < numberOfThreads; i++ {
		balancer.ch <- ColumnSlot{
			Start: columnName(i * columnsPerSlot),
			End:   columnName((i+1)*columnsPerSlot - 1),
		}
	}

	return balancer, nil
}

func (b *GoogleFinanceBalancer) Acquire(ctx context.Context) (ColumnSlot, error) {
	select {
	case slot := <-b.ch:
		return slot, nil
	case <-ctx.Done():
		return ColumnSlot{}, ctx.Err()
	}
}

func (b *GoogleFinanceBalancer) Release(slot ColumnSlot) {
	b.ch <- slot
}
