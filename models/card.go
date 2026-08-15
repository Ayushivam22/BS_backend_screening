package models

// CardPayment matches the "credit_card" group payload.
type CardPayment struct {
	Card struct {
		Channel     string `json:"channel"`
		CardNumber  string `json:"card_number"`
		CardNetwork string `json:"card_network"`
		CardType    string `json:"card_type"`
		BankName    string `json:"card_bank_name"`
	} `json:"card"`
}

// GetPaymentGroup returns the identifier for credit card payments.
func (c *CardPayment) GetPaymentGroup() string {
	return "credit_card"
}
