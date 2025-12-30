package orders

import (
	"errors"
	"sync"
)

// NewInMemoryPaymentClient constructs an in-memory payment client.
func NewInMemoryPaymentClient() *InMemoryPaymentClient {
	return &InMemoryPaymentClient{
		charges:  make(map[string]float64),
		refunds:  make(map[string]float64),
		refunded: make(map[string]bool),
	}
}

// InMemoryPaymentClient tracks charges and refunds in memory.
type InMemoryPaymentClient struct {
	mu       sync.Mutex
	charges  map[string]float64
	refunds  map[string]float64
	refunded map[string]bool
}

func (c *InMemoryPaymentClient) Charge(orderID string, amount float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.charges[orderID] = amount
	return nil
}

func (c *InMemoryPaymentClient) Refund(orderID string, amount float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.charges[orderID]; !ok {
		return errors.New("refund without charge")
	}
	c.refunds[orderID] = amount
	c.refunded[orderID] = true
	return nil
}

// WasCharged reports whether an order was charged (for testing/inspection).
func (c *InMemoryPaymentClient) WasCharged(orderID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.charges[orderID]
	return ok
}

// WasRefunded reports whether an order was refunded (for testing/inspection).
func (c *InMemoryPaymentClient) WasRefunded(orderID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.refunded[orderID]
}

// NewInMemoryDriverClient constructs an in-memory driver client.
func NewInMemoryDriverClient() *InMemoryDriverClient {
	return &InMemoryDriverClient{
		assignments: make(map[string]string),
	}
}

// InMemoryDriverClient tracks driver assignments in memory.
type InMemoryDriverClient struct {
	mu          sync.Mutex
	assignments map[string]string
}

func (c *InMemoryDriverClient) Assign(orderID string, driverID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.assignments[orderID] = driverID
	return nil
}

// Assignment returns the driver assigned to an order, if any.
func (c *InMemoryDriverClient) Assignment(orderID string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	driver, ok := c.assignments[orderID]
	return driver, ok
}
