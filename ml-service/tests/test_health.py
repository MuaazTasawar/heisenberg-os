import pytest

ENDPOINTS = [
    ("/ml/threat/analyze", {"empire_id": 1, "heat_level": 10, "flagged_transactions": 0,
                            "batch_yield": 10, "territory_count": 0, "days_active": 1}),
    ("/ml/yield/optimize", {"current_purity": 50, "effort_level": 50,
                            "supply_quality": 80, "lab_capacity": 100}),
    ("/ml/laundering/analyze", {"amounts": [1000], "business_revenues": [5000],
                                "time_gaps": [24], "business_type": "restaurant"}),
    ("/ml/evidence", {"empire_id": 424242, "source": "tx:health", "weight": 1}),
]


def test_health_is_open_and_reports_version(client):
    body = client.get("/health").json()
    assert body["status"] == "alive"
    assert body["version"] == "2.0.0"
    assert body["auth_required"] is False


def test_health_reports_when_a_key_is_required(client, monkeypatch):
    monkeypatch.setenv("ML_API_KEY", "s3cret")
    assert client.get("/health").json()["auth_required"] is True


@pytest.mark.parametrize("path,payload", ENDPOINTS)
def test_endpoints_are_open_when_no_key_is_configured(client, path, payload):
    assert client.post(path, json=payload).status_code in (200, 201)


@pytest.mark.parametrize("path,payload", ENDPOINTS)
def test_missing_key_is_rejected(client, monkeypatch, path, payload):
    monkeypatch.setenv("ML_API_KEY", "s3cret")
    assert client.post(path, json=payload).status_code == 401


@pytest.mark.parametrize("path,payload", ENDPOINTS)
def test_wrong_key_is_rejected(client, monkeypatch, path, payload):
    monkeypatch.setenv("ML_API_KEY", "s3cret")
    assert client.post(path, json=payload, headers={"X-API-Key": "nope"}).status_code == 401


@pytest.mark.parametrize("path,payload", ENDPOINTS)
def test_correct_key_is_accepted(client, monkeypatch, path, payload):
    monkeypatch.setenv("ML_API_KEY", "s3cret")
    response = client.post(path, json=payload, headers={"X-API-Key": "s3cret"})
    assert response.status_code in (200, 201)


def test_health_stays_open_even_when_a_key_is_required(client, monkeypatch):
    monkeypatch.setenv("ML_API_KEY", "s3cret")
    assert client.get("/health").status_code == 200


def test_evidence_lookup_is_protected_too(client, monkeypatch):
    monkeypatch.setenv("ML_API_KEY", "s3cret")
    assert client.get("/ml/evidence/1").status_code == 401
    assert client.get("/ml/evidence/1", headers={"X-API-Key": "s3cret"}).status_code == 200


def test_no_cors_headers_by_default(client):
    response = client.get("/health", headers={"Origin": "http://evil.example"})
    assert "access-control-allow-origin" not in response.headers
