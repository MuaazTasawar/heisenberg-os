package models

import "gorm.io/gorm"

type LegalCase struct {
	gorm.Model
	EmpireID uint   `json:"empire_id"`
	Charge   string `json:"charge"`
	Evidence string `json:"evidence"`
	Status   string `json:"status"` // open, dismissed, convicted
	Severity int    `json:"severity"`
}

type DEAThreat struct {
	gorm.Model
	EmpireID      uint    `json:"empire_id"`
	ThreatScore   float64 `json:"threat_score"`
	EvidenceCount int     `json:"evidence_count"`
	LeadAgent     string  `json:"lead_agent"`
	IsActive      bool    `json:"is_active"`
}
