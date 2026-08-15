package models

// UPIPayment matches the "upi" group payload.
type UPIPayment struct {
	UPI struct {
		Channel string `json:"channel"`
		UPIID   string `json:"upi_id"`
	} `json:"upi"`
}

// GetPaymentGroup returns the identifier for UPI payments.
func (u *UPIPayment) GetPaymentGroup() string {
	return "upi"
}
