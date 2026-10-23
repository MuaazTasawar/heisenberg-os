from typing import Annotated, List, Optional

from pydantic import BaseModel, Field


# A dollar amount or an hour count: finite and never negative.
NonNegative = Annotated[float, Field(ge=0, le=1e12, allow_inf_nan=False)]


# ---------------------------------------------------------------- DEA threat
class ThreatInput(BaseModel):
    empire_id: int = Field(ge=0)
    heat_level: float = Field(ge=0, le=1000, allow_inf_nan=False)
    flagged_transactions: int = Field(ge=0, le=100_000)
    batch_yield: float = Field(ge=0, le=1e9, allow_inf_nan=False)
    territory_count: int = Field(ge=0, le=1000)
    days_active: int = Field(ge=0, le=100_000)


class ThreatOutput(BaseModel):
    threat_score: float
    risk_level: str
    prediction: str
    recommendation: str


# ------------------------------------------------------------ DEA evidence
class EvidenceIn(BaseModel):
    empire_id: int = Field(ge=0)
    source: str = Field(min_length=1, max_length=80)
    weight: float = Field(default=1.0, gt=0, le=1000, allow_inf_nan=False)
    kind: str = Field(default="transaction", min_length=1, max_length=40)


class EvidenceSource(BaseModel):
    source: str
    kind: str
    weight: float


class EvidenceSummary(BaseModel):
    empire_id: int
    node_count: int
    total_weight: float
    top_sources: List[EvidenceSource]


# ------------------------------------------------------------------- yield
class YieldInput(BaseModel):
    """effort_level is on a 0-100 scale (the Go API sends cook effort 1-10 multiplied by 10)."""

    current_purity: float = Field(ge=0, le=100, allow_inf_nan=False)
    effort_level: float = Field(ge=0, le=100, allow_inf_nan=False)
    supply_quality: float = Field(ge=0, le=100, allow_inf_nan=False)
    lab_capacity: float = Field(gt=0, le=1e6, allow_inf_nan=False)


class YieldOutput(BaseModel):
    predicted_yield: float
    optimal_effort: float
    optimal_yield: float
    purity_projection: float
    efficiency_score: float


# --------------------------------------------------------------- laundering
class TransactionPattern(BaseModel):
    """amounts in dollars, business_revenues in dollars, time_gaps in hours."""

    amounts: List[NonNegative] = Field(max_length=1000)
    business_revenues: List[NonNegative] = Field(default_factory=list, max_length=1000)
    time_gaps: List[NonNegative] = Field(default_factory=list, max_length=1000)
    business_type: str = Field(default="other", max_length=50)


class LaunderingRisk(BaseModel):
    risk_score: float
    anomaly_detected: bool
    suspicious_patterns: List[str]
    recommendation: str
    revenue_ratio: Optional[float] = None
