package models

// NetBankingPayment matches the "net_banking" group payload.
type NetBankingPayment struct {
	NetBanking struct {
		Channel  string `json:"channel"`
		BankName string `json:"netbanking_bank_name"`
	} `json:"netbanking"`
}

// GetPaymentGroup returns the identifier for Net Banking payments.
func (n *NetBankingPayment) GetPaymentGroup() string {
	return "net_banking"
}
