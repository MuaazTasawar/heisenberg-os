from fastapi import APIRouter, Path

from app.models.schemas import EvidenceIn, EvidenceSummary
from app.services.dea_agent import dea_agent

router = APIRouter()


@router.post("", response_model=EvidenceSummary, status_code=201)
def add_evidence(data: EvidenceIn):
    """Record one piece of evidence (for example a flagged transaction) against an empire."""
    dea_agent.evidence.add(data.empire_id, data.source, data.weight, data.kind)
    return dea_agent.evidence.summary(data.empire_id)


@router.get("/{empire_id}", response_model=EvidenceSummary)
def get_evidence(empire_id: int = Path(ge=0)):
    """The DEA's evidence file on one empire: strongest sources first."""
    return dea_agent.evidence.summary(empire_id)
