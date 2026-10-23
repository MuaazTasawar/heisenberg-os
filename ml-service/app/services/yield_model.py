"""Cook-yield model.

yield(x) = base * x * (1 - 0.6 * x)       with x = effort / 100

The linear term is the output of the lab; the quadratic term is waste from rushing the
batch. The two balance at x = 1 / (2 * 0.6) ≈ 0.833, so the best effort sits *inside* the
0-100 range instead of always being "as much as possible".
"""
from scipy.optimize import minimize_scalar

OVERHEAT_PENALTY = 0.6
MIN_PURITY = 20.0
MAX_PURITY = 99.1


def purity_delta(effort10: float) -> float:
    """Purity change for one batch, with effort on the 0-10 scale.

    Careful cooking (effort <= 7) improves purity by 0.6 points; from 7 to 10 the gain
    falls linearly to a loss of 0.8 points (rushed, contaminated batches). The Go backend
    applies exactly the same rule.
    """
    effort10 = min(max(effort10, 0.0), 10.0)
    if effort10 <= 7.0:
        return 0.6
    return 0.6 - (effort10 - 7.0) * (1.4 / 3.0)


class YieldOptimizer:
    @staticmethod
    def _yield_at(purity: float, effort: float, supply: float, capacity: float) -> float:
        x = min(max(effort, 0.0), 100.0) / 100.0
        purity_factor = 1 + (purity - 50) / 200
        base = capacity * (supply / 100) * purity_factor
        return base * x * (1 - OVERHEAT_PENALTY * x)

    def predict_yield(self, purity: float, effort: float, supply: float, capacity: float) -> dict:
        predicted = self._yield_at(purity, effort, supply, capacity)

        if supply <= 0:
            # Nothing to cook with: every effort level yields zero.
            optimal_effort, optimal_yield = 0.0, 0.0
        else:
            result = minimize_scalar(
                lambda e: -self._yield_at(purity, e, supply, capacity),
                bounds=(0.0, 100.0),
                method="bounded",
                options={"xatol": 1e-6},
            )
            optimal_effort = float(result.x)
            optimal_yield = self._yield_at(purity, optimal_effort, supply, capacity)

        efficiency = min(100.0, predicted / optimal_yield * 100) if optimal_yield > 0 else 0.0
        projected = purity + (purity_delta(optimal_effort / 10.0) if optimal_yield > 0 else 0.0)
        projected = min(max(projected, MIN_PURITY), MAX_PURITY)

        return {
            "predicted_yield": round(predicted, 2),
            "optimal_effort": round(optimal_effort, 2),
            "optimal_yield": round(optimal_yield, 2),
            "purity_projection": round(projected, 2),
            "efficiency_score": round(efficiency, 2),
        }


yield_optimizer = YieldOptimizer()
