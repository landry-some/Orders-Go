package orders

// PaymentClient charges a payment instrument for an order.
type PaymentClient interface {
	Charge(orderID string, amount float64) error
}

// DriverClient assigns a driver to an order.
type DriverClient interface {
	Assign(orderID string, driverID string) error
}

// OrderService coordinates payment and driver assignment.
type OrderService struct {
	payments PaymentClient
	drivers  DriverClient
}

// NewOrderService constructs an OrderService.
func NewOrderService(payments PaymentClient, drivers DriverClient) *OrderService {
	return &OrderService{
		payments: payments,
		drivers:  drivers,
	}
}

// CreateOrder orchestrates the payment and driver assignment steps.
func (s *OrderService) CreateOrder(orderID string, amount float64, driverID string) error {
	if err := s.payments.Charge(orderID, amount); err != nil {
		return err
	}

	if err := s.drivers.Assign(orderID, driverID); err != nil {
		return err
	}

	return nil
}
