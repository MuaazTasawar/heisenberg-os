from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from app.routers import threat, yield_optimizer, laundering

app = FastAPI(title="Heisenberg OS — ML Service", version="1.0.0")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(threat.router, prefix="/ml/threat", tags=["DEA Threat"])
app.include_router(yield_optimizer.router, prefix="/ml/yield", tags=["Yield Optimizer"])
app.include_router(laundering.router, prefix="/ml/laundering", tags=["Laundering Detector"])

@app.get("/health")
def health():
    return {"status": "alive", "message": "I am the danger"}