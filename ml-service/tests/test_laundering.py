import pytest

STRUCTURING = {
    "amounts": [9500, 9500, 9500, 9500],
    "business_revenues": [12000, 11800, 12100, 12050],
    "time_gaps": [0.5, 0.4, 0.6, 0.5],
    "business_type": "car_wash",
}
BENIGN = {
    "amounts": [1200, 3400, 800, 2100],
    "business_revenues": [5000, 5200, 4800, 5100],
    "time_gaps": [24, 30, 18, 26],
    "business_type": "restaurant",
}


def analyze(client, payload):
    response = client.post("/ml/laundering/analyze", json=payload)
    assert response.status_code == 200, response.text
    return response.json()


def test_no_transactions_means_no_data(client):
    result = analyze(client, {"amounts": [], "business_revenues": [], "time_gaps": [], "business_type": "x"})
    assert result["risk_score"] == 0
    assert result["recommendation"] == "No data"
    assert result["revenue_ratio"] is None
    assert result["suspicious_patterns"] == []


def test_structuring_is_caught(client):
    result = analyze(client, STRUCTURING)
    patterns = " | ".join(result["suspicious_patterns"])
    assert "Structuring" in patterns
    assert "High-frequency" in patterns
    assert result["anomaly_detected"] is True
    assert result["risk_score"] >= 80
    assert result["recommendation"] == "Reduce transaction sizes"


def test_ordinary_activity_is_left_alone(client):
    result = analyze(client, BENIGN)
    assert result["anomaly_detected"] is False
    assert result["suspicious_patterns"] == []
    assert result["risk_score"] == 0
    assert result["recommendation"] == "Patterns look normal"


def test_one_huge_transfer_is_flagged(client):
    result = analyze(client, {"amounts": [120000], "business_revenues": [], "time_gaps": [20],
                              "business_type": "restaurant"})
    assert "Large single transaction detected" in result["suspicious_patterns"]
    assert result["anomaly_detected"] is True


def test_revenue_ratio_is_used(client):
    result = analyze(client, {"amounts": [9000], "business_revenues": [10000], "time_gaps": [20],
                              "business_type": "restaurant"})
    assert result["revenue_ratio"] == pytest.approx(0.9)
    assert any("90% of declared revenue" in p for p in result["suspicious_patterns"])


def test_small_share_of_revenue_is_fine(client):
    result = analyze(client, {"amounts": [2000], "business_revenues": [10000], "time_gaps": [20],
                              "business_type": "restaurant"})
    assert result["revenue_ratio"] == pytest.approx(0.2)
    assert not any("declared revenue" in p for p in result["suspicious_patterns"])


def test_revenue_ratio_is_none_without_revenue_data(client):
    assert analyze(client, {**BENIGN, "business_revenues": []})["revenue_ratio"] is None
    assert analyze(client, {**BENIGN, "business_revenues": [0, 0]})["revenue_ratio"] is None


def test_small_business_with_big_transfers_is_inconsistent(client):
    payload = {"amounts": [25000, 31000, 28000], "business_revenues": [], "time_gaps": [30, 40, 20],
               "business_type": "Nail_Salon"}
    assert "Revenue inconsistent with business type" in analyze(client, payload)["suspicious_patterns"]


def test_time_gaps_and_type_are_optional(client):
    result = analyze(client, {"amounts": [1000, 2500]})
    assert result["risk_score"] >= 0


def test_risk_score_is_capped_at_100(client):
    payload = {"amounts": [90000] * 5, "business_revenues": [20000] * 5, "time_gaps": [0.1] * 5,
               "business_type": "car_wash"}
    assert analyze(client, payload)["risk_score"] == 100


def test_same_input_gives_the_same_answer(client):
    assert analyze(client, STRUCTURING) == analyze(client, STRUCTURING)


@pytest.mark.parametrize(
    "overrides",
    [
        {"amounts": [-5]},
        {"amounts": [1] * 1001},
        {"business_revenues": [-1]},
        {"time_gaps": [-0.5]},
        {"business_type": "x" * 51},
        {"amounts": "lots"},
    ],
)
def test_invalid_input_is_rejected(client, overrides):
    assert client.post("/ml/laundering/analyze", json={**BENIGN, **overrides}).status_code == 422
