import pytest

from app.services.dea_agent import FEATURE_WEIGHTS, dea_agent, risk_band

BASE = {
    "empire_id": 1,
    "heat_level": 0,
    "flagged_transactions": 0,
    "batch_yield": 0,
    "territory_count": 0,
    "days_active": 0,
}
FEATURES = ["heat_level", "flagged_transactions", "batch_yield", "territory_count", "days_active"]


def analyze(client, **overrides):
    response = client.post("/ml/threat/analyze", json={**BASE, **overrides})
    assert response.status_code == 200, response.text
    return response.json()


def test_calm_empire_is_low(client):
    result = analyze(client)
    assert result["risk_level"] == "low"
    assert result["threat_score"] < 10


def test_maxed_out_empire_is_critical(client):
    result = analyze(client, heat_level=100, flagged_transactions=20, batch_yield=1000,
                     territory_count=10, days_active=365)
    assert result["risk_level"] == "critical"
    assert result["threat_score"] > 90


@pytest.mark.parametrize(
    "state,level",
    [
        ({"heat_level": 5, "batch_yield": 10, "days_active": 3}, "low"),
        ({"heat_level": 50, "flagged_transactions": 10, "batch_yield": 300,
          "territory_count": 3, "days_active": 60}, "medium"),
        ({"heat_level": 70, "flagged_transactions": 12, "batch_yield": 400,
          "territory_count": 4, "days_active": 120}, "high"),
        ({"heat_level": 80, "flagged_transactions": 15, "batch_yield": 600,
          "territory_count": 5, "days_active": 200}, "critical"),
    ],
)
def test_every_risk_level_is_reachable(client, state, level):
    assert analyze(client, **state)["risk_level"] == level


def test_a_brand_new_empire_at_full_heat_is_not_ignored(client):
    # Corner of the feature space: this is what four max-effort cooks look like.
    result = analyze(client, heat_level=100, batch_yield=110)
    assert result["threat_score"] >= 15


def test_scores_are_graded_not_binary(client):
    scores = {analyze(client, heat_level=h, flagged_transactions=h // 5)["threat_score"]
              for h in range(0, 101, 10)}
    assert len(scores) >= 8
    assert any(10 < s < 90 for s in scores)


@pytest.mark.parametrize(
    "score,level",
    [(0, "low"), (24.99, "low"), (25, "medium"), (49.99, "medium"),
     (50, "high"), (74.99, "high"), (75, "critical"), (100, "critical")],
)
def test_risk_band_boundaries(score, level):
    assert risk_band(score)[0] == level


def test_response_level_matches_its_own_score(client):
    for heat in (0, 30, 60, 90):
        result = analyze(client, heat_level=heat, flagged_transactions=heat // 6, batch_yield=heat * 5)
        assert risk_band(result["threat_score"])[0] == result["risk_level"]


def test_same_input_always_gives_the_same_score(client):
    payload = {"heat_level": 61.5, "flagged_transactions": 4, "batch_yield": 120,
               "territory_count": 3, "days_active": 90}
    assert len({analyze(client, **payload)["threat_score"] for _ in range(5)}) == 1


@pytest.mark.parametrize("feature,top", [("heat_level", 100), ("flagged_transactions", 20),
                                          ("batch_yield", 1000), ("territory_count", 10),
                                          ("days_active", 365)])
def test_threat_never_decreases_when_a_feature_grows(client, feature, top):
    mid = {"heat_level": 40, "flagged_transactions": 6, "batch_yield": 300,
           "territory_count": 3, "days_active": 120}
    previous = -1.0
    for step in range(0, 11):
        value = top * step / 10
        if feature in ("flagged_transactions", "territory_count", "days_active"):
            value = int(value)
        score = analyze(client, **{**mid, feature: value})["threat_score"]
        assert score >= previous - 1e-9, f"{feature} at {value} lowered the score"
        previous = score


def test_all_learned_coefficients_are_positive():
    assert (dea_agent.model.coef_ > 0).all()
    assert len(dea_agent.model.coef_[0]) == len(FEATURE_WEIGHTS)


def test_values_beyond_the_scale_are_clamped(client):
    assert analyze(client, heat_level=100)["threat_score"] == analyze(client, heat_level=900)["threat_score"]


@pytest.mark.parametrize(
    "overrides",
    [
        {"heat_level": -1},
        {"heat_level": 1001},
        {"flagged_transactions": -3},
        {"batch_yield": -0.5},
        {"territory_count": -1},
        {"days_active": -1},
        {"empire_id": -1},
        {"heat_level": "hot"},
    ],
)
def test_invalid_input_is_rejected(client, overrides):
    assert client.post("/ml/threat/analyze", json={**BASE, **overrides}).status_code == 422


@pytest.mark.parametrize("feature", FEATURES)
def test_missing_field_is_rejected(client, feature):
    payload = {k: v for k, v in BASE.items() if k != feature}
    assert client.post("/ml/threat/analyze", json=payload).status_code == 422


def test_recommendation_and_prediction_are_filled_in(client):
    result = analyze(client, heat_level=90, flagged_transactions=15, batch_yield=700,
                     territory_count=6, days_active=250)
    assert result["prediction"] and result["recommendation"]
