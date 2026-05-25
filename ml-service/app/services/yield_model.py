import numpy as np
from scipy.optimize import minimize

class YieldOptimizer:
    def predict_yield(self, purity: float, effort: float, supply: float, capacity: float) -> dict:
        base_yield = capacity * (supply / 100) * (effort / 100)
        purity_factor = 1 + (purity - 50) / 200
        predicted = round(base_yield * purity_factor, 2)
        efficiency = round((predicted / capacity) * 100, 2) if capacity > 0 else 0

        result = minimize(
            lambda e: -self._yield_fn(purity, e[0], supply, capacity),
            x0=[effort],
            bounds=[(0, 100)]
        )
        optimal_effort = round(result.x[0], 2)
        purity_proj = min(100, round(purity + (optimal_effort * 0.05), 2))

        return {
            "predicted_yield": predicted,
            "optimal_effort": optimal_effort,
            "purity_projection": purity_proj,
            "efficiency_score": efficiency
        }

    def _yield_fn(self, purity, effort, supply, capacity):
        base = capacity * (supply / 100) * (effort / 100)
        return base * (1 + (purity - 50) / 200)

yield_optimizer = YieldOptimizer()