package models

const (
	TxDirty     = "dirty"
	TxClean     = "clean"
	TxLaundered = "laundered"
)

type Transaction struct {
	Base
	EmpireID    uint    `gorm:"index" json:"empire_id"`
	Amount      float64 `json:"amount"`
	Type        string  `json:"type"` // dirty, clean, laundered
	Description string  `json:"description"`
	BusinessID  uint    `json:"business_id"`
	Flagged     bool    `json:"flagged"`
	FlagReason  string  `json:"flag_reason,omitempty"`
	RiskScore   float64 `json:"risk_score"`
}
