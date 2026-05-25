from fastapi import APIRouter
from app.models.schemas import YieldInput, YieldOutput
from app.services.yield_model import yield_optimizer

router = APIRouter()

@router.post("/optimize", response_model=YieldOutput)
def optimize_yield(data: YieldInput):
    result = yield_optimizer.predict_yield(
        data.current_purity,
        data.effort_level,
        data.supply_quality,
        data.lab_capacity
    )
    return result