package models

// PaymentMethod is the contract that any specific payment method struct must implement.
type PaymentMethod interface {
	GetPaymentGroup() string
}
