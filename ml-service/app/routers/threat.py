from fastapi import APIRouter
from app.models.schemas import ThreatInput, ThreatOutput
from app.services.dea_agent import dea_agent

router = APIRouter()

@router.post("/analyze", response_model=ThreatOutput)
def analyze_threat(data: ThreatInput):
    result = dea_agent.calculate_threat(data.dict())
    return result