package balancer

import "fmt"

var elements = [...]string{"A", "B", "C", "D", "E", "F"}

type GoogleFinanceBalancer struct {
	numberOfThreads int

	ch chan string
}

func NewGoogleFinanceBalancer(numberOfThreads int) (*GoogleFinanceBalancer, error) {
	if numberOfThreads > len(elements) {
		return nil, fmt.Errorf("NewGoogleFinanceBalancer: currently only %d threads is supported, %d given", len(elements), numberOfThreads)
	}

	if numberOfThreads <= 0 {
		return nil, fmt.Errorf("NewGoogleFinanceBalancer: numberOfThreads should be > 0, %d given", numberOfThreads)
	}

	balancer := &GoogleFinanceBalancer{
		numberOfThreads: numberOfThreads,
		ch:              make(chan string, numberOfThreads),
	}

	for _, elem := range elements[:numberOfThreads] {
		balancer.ch <- elem
	}

	return balancer, nil
}

func (b *GoogleFinanceBalancer) Acquire() string {
	return <-b.ch
}

func (b *GoogleFinanceBalancer) Release(element string) {
	b.ch <- element
}
