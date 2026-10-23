import os

from fastapi import Depends, FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.routers import evidence, laundering, threat, yield_optimizer
from app.security import require_api_key

VERSION = "2.0.0"

app = FastAPI(title="Heisenberg OS — ML Service", version=VERSION)

# The Go API calls this service server-to-server, so browsers never need CORS access.
# Set ALLOWED_ORIGINS (comma separated) only if you want to call it from a web page.
_origins = [o.strip() for o in os.environ.get("ALLOWED_ORIGINS", "").split(",") if o.strip()]
if _origins:
    app.add_middleware(
        CORSMiddleware,
        allow_origins=_origins,
        allow_methods=["GET", "POST"],
        allow_headers=["Content-Type", "X-API-Key"],
    )

_secured = [Depends(require_api_key)]

app.include_router(threat.router, prefix="/ml/threat", tags=["DEA Threat"], dependencies=_secured)
app.include_router(evidence.router, prefix="/ml/evidence", tags=["DEA Evidence"], dependencies=_secured)
app.include_router(yield_optimizer.router, prefix="/ml/yield", tags=["Yield Optimizer"], dependencies=_secured)
app.include_router(laundering.router, prefix="/ml/laundering", tags=["Laundering Detector"], dependencies=_secured)


@app.get("/health", tags=["Health"])
def health():
    return {
        "status": "alive",
        "message": "I am the danger",
        "version": VERSION,
        "auth_required": bool(os.environ.get("ML_API_KEY")),
    }
