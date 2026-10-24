package models

const (
	CaseOpen      = "open"
	CaseDismissed = "dismissed"
	CaseConvicted = "convicted"
)

type LegalCase struct {
	Base
	EmpireID uint   `gorm:"index" json:"empire_id"`
	Charge   string `json:"charge"`
	Evidence string `gorm:"type:text" json:"evidence"`
	Status   string `json:"status"` // open, dismissed, convicted
	Severity int    `json:"severity"`
	Risk     string `json:"risk"`
	Advice   string `gorm:"type:text" json:"advice"`
	Defense  string `gorm:"type:text" json:"defense"`
}

type DEAThreat struct {
	Base
	EmpireID       uint    `gorm:"index" json:"empire_id"`
	ThreatScore    float64 `json:"threat_score"`
	EvidenceCount  int     `json:"evidence_count"`
	LeadAgent      string  `json:"lead_agent"`
	IsActive       bool    `json:"is_active"`
	RiskLevel      string  `json:"risk_level"`
	Prediction     string  `json:"prediction"`
	Recommendation string  `json:"recommendation"`
	Source         string  `json:"source"` // "ml" or "heuristic"
}
