from pydantic import BaseModel
from typing import List, Optional

class ThreatInput(BaseModel):
    empire_id: int
    heat_level: float
    flagged_transactions: int
    batch_yield: float
    territory_count: int
    days_active: int

class ThreatOutput(BaseModel):
    threat_score: float
    risk_level: str
    prediction: str
    recommendation: str

class YieldInput(BaseModel):
    current_purity: float
    effort_level: float
    supply_quality: float
    lab_capacity: float

class YieldOutput(BaseModel):
    predicted_yield: float
    optimal_effort: float
    purity_projection: float
    efficiency_score: float

class TransactionPattern(BaseModel):
    amounts: List[float]
    business_revenues: List[float]
    time_gaps: List[float]
    business_type: str

class LaunderingRisk(BaseModel):
    risk_score: float
    anomaly_detected: bool
    suspicious_patterns: List[str]
    recommendation: str