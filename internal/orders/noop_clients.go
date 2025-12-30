package orders

// NoopPaymentClient is a stub PaymentClient that always succeeds.
type NoopPaymentClient struct{}

func (n *NoopPaymentClient) Charge(orderID string, amount float64) error {
	return nil
}

func (n *NoopPaymentClient) Refund(orderID string, amount float64) error {
	return nil
}

// NoopDriverClient is a stub DriverClient that always succeeds.
type NoopDriverClient struct{}

func (n *NoopDriverClient) Assign(orderID string, driverID string) error {
	return nil
}
