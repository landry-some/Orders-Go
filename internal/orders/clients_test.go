package orders

import "testing"

func TestInMemoryPaymentClient_ChargeThenRefund(t *testing.T) {
	p := NewInMemoryPaymentClient()

	if err := p.Charge("order-1", 9.99); err != nil {
		t.Fatalf("charge: %v", err)
	}

	if !p.WasCharged("order-1") {
		t.Fatalf("expected order-1 to be charged")
	}

	if err := p.Refund("order-1", 9.99); err != nil {
		t.Fatalf("refund: %v", err)
	}

	if !p.WasRefunded("order-1") {
		t.Fatalf("expected order-1 to be refunded")
	}
}

func TestInMemoryPaymentClient_RefundWithoutChargeFails(t *testing.T) {
	p := NewInMemoryPaymentClient()

	if err := p.Refund("order-unknown", 1.23); err == nil {
		t.Fatalf("expected refund to fail for unknown order")
	}
}

func TestInMemoryDriverClient_AssignStoresMapping(t *testing.T) {
	d := NewInMemoryDriverClient()

	if err := d.Assign("order-1", "driver-123"); err != nil {
		t.Fatalf("assign: %v", err)
	}

	driverID, ok := d.Assignment("order-1")
	if !ok {
		t.Fatalf("expected assignment for order-1")
	}

	if driverID != "driver-123" {
		t.Fatalf("unexpected driver: got %s, want driver-123", driverID)
	}
}
