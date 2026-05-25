from fastapi import APIRouter
from app.models.schemas import TransactionPattern, LaunderingRisk
from app.services.laundering_detector import laundering_detector

router = APIRouter()

@router.post("/analyze", response_model=LaunderingRisk)
def analyze_laundering(data: TransactionPattern):
    result = laundering_detector.analyze(
        data.amounts,
        data.business_revenues,
        data.time_gaps,
        data.business_type
    )
    return result