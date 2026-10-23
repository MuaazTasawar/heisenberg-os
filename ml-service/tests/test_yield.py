import pytest

from app.services.yield_model import MAX_PURITY, MIN_PURITY, purity_delta, yield_optimizer

PAYLOAD = {"current_purity": 50, "effort_level": 60, "supply_quality": 80, "lab_capacity": 100}


def optimize(client, **overrides):
    response = client.post("/ml/yield/optimize", json={**PAYLOAD, **overrides})
    assert response.status_code == 200, response.text
    return response.json()


def test_known_example(client):
    result = optimize(client)
    assert result["predicted_yield"] == pytest.approx(30.72, abs=0.01)
    assert result["optimal_effort"] == pytest.approx(83.33, abs=0.05)
    assert result["optimal_yield"] == pytest.approx(33.33, abs=0.02)
    assert result["efficiency_score"] == pytest.approx(92.16, abs=0.1)
    assert result["purity_projection"] == pytest.approx(49.98, abs=0.05)


def test_optimum_is_inside_the_range_not_at_the_edge(client):
    for effort in (0, 25, 60, 100):
        assert 80 < optimize(client, effort_level=effort)["optimal_effort"] < 87


def test_optimal_effort_does_not_depend_on_supply_or_capacity(client):
    assert optimize(client, supply_quality=30)["optimal_effort"] == pytest.approx(
        optimize(client, lab_capacity=500)["optimal_effort"], abs=0.05)


def test_optimum_beats_every_other_effort(client):
    best = optimize(client)["optimal_yield"]
    for effort in range(0, 101, 5):
        assert optimize(client, effort_level=effort)["predicted_yield"] <= best + 0.01


def test_yield_grows_with_effort_then_falls_when_rushed(client):
    low = optimize(client, effort_level=30)["predicted_yield"]
    sweet = optimize(client, effort_level=83)["predicted_yield"]
    rushed = optimize(client, effort_level=100)["predicted_yield"]
    assert low < sweet and rushed < sweet


def test_efficiency_is_100_at_the_optimum_and_never_above(client):
    assert optimize(client, effort_level=83.33)["efficiency_score"] == pytest.approx(100, abs=0.1)
    for effort in range(0, 101, 10):
        assert optimize(client, effort_level=effort)["efficiency_score"] <= 100


def test_purer_product_yields_more(client):
    assert optimize(client, current_purity=90)["predicted_yield"] > optimize(client, current_purity=50)["predicted_yield"]


def test_no_supply_means_no_yield(client):
    result = optimize(client, supply_quality=0)
    assert result["predicted_yield"] == 0
    assert result["optimal_effort"] == 0
    assert result["optimal_yield"] == 0
    assert result["efficiency_score"] == 0
    assert result["purity_projection"] == 50


@pytest.mark.parametrize(
    "overrides",
    [{"effort_level": 101}, {"effort_level": -1}, {"current_purity": 101}, {"supply_quality": -5},
     {"lab_capacity": 0}, {"lab_capacity": -10}, {"lab_capacity": 1e9}],
)
def test_invalid_input_is_rejected(client, overrides):
    assert client.post("/ml/yield/optimize", json={**PAYLOAD, **overrides}).status_code == 422


@pytest.mark.parametrize("effort10,delta", [(0, 0.6), (5, 0.6), (7, 0.6), (8.5, 0.6 - 1.5 * 1.4 / 3), (10, -0.8)])
def test_purity_delta_known_points(effort10, delta):
    assert purity_delta(effort10) == pytest.approx(delta)


def test_purity_delta_never_increases_with_effort():
    values = [purity_delta(e / 10) for e in range(0, 101)]
    assert all(a >= b - 1e-12 for a, b in zip(values, values[1:]))


def test_purity_delta_clamps_out_of_range_effort():
    assert purity_delta(-5) == purity_delta(0)
    assert purity_delta(50) == purity_delta(10)


def test_purity_projection_stays_within_bounds():
    low = yield_optimizer.predict_yield(MIN_PURITY, 100, 80, 100)["purity_projection"]
    high = yield_optimizer.predict_yield(100, 20, 80, 100)["purity_projection"]
    assert MIN_PURITY <= low <= MAX_PURITY
    assert high <= MAX_PURITY
