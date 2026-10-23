import pytest
from fastapi.testclient import TestClient

from app.main import app


@pytest.fixture(scope="session")
def client() -> TestClient:
    return TestClient(app)


@pytest.fixture(autouse=True)
def _auth_disabled_by_default(monkeypatch):
    """Tests run without the shared secret unless they set ML_API_KEY themselves."""
    monkeypatch.delenv("ML_API_KEY", raising=False)
