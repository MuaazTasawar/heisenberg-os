"""Laundering anomaly detector: an IsolationForest baseline plus human-readable rules."""
from typing import List, Optional

import numpy as np
from sklearn.ensemble import IsolationForest

LARGE_TRANSACTION = 50_000
STRUCTURING_STD = 100
HIGH_FREQUENCY_HOURS = 1
REVENUE_RATIO_LIMIT = 0.5
SMALL_BUSINESS_LIMIT = 20_000
SMALL_BUSINESSES = {"car_wash", "nail_salon"}


class LaunderingDetector:
    def __init__(self) -> None:
        self.model = IsolationForest(contamination=0.05, random_state=42)
        self._fit_baseline()

    def _fit_baseline(self) -> None:
        # "Normal" behaviour: modest amounts whose spread scales with their size
        # (real takings vary), spaced hours apart. Identical amounts or rapid-fire
        # transfers fall far outside this cloud.
        rng = np.random.default_rng(42)
        mean_amount = rng.exponential(scale=5000, size=500) + 500
        spread = mean_amount * rng.uniform(0.3, 1.3, size=500)
        gap_hours = rng.uniform(2, 72, size=500)
        self.model.fit(np.column_stack([mean_amount, spread, gap_hours]))

    def analyze(
        self,
        amounts: List[float],
        revenues: List[float],
        time_gaps: List[float],
        business_type: str,
    ) -> dict:
        if not amounts:
            return {
                "risk_score": 0.0,
                "anomaly_detected": False,
                "suspicious_patterns": [],
                "recommendation": "No data",
                "revenue_ratio": None,
            }

        mean_amount = float(np.mean(amounts))
        spread = float(np.std(amounts)) if len(amounts) > 1 else 0.0
        mean_gap = float(np.mean(time_gaps)) if time_gaps else 24.0

        anomaly = bool(self.model.predict(np.array([[mean_amount, spread, mean_gap]]))[0] == -1)

        revenue_ratio: Optional[float] = None
        if revenues and float(np.mean(revenues)) > 0:
            revenue_ratio = round(mean_amount / float(np.mean(revenues)), 4)

        patterns = []
        if max(amounts) > LARGE_TRANSACTION:
            patterns.append("Large single transaction detected")
        if len(amounts) > 1 and spread < STRUCTURING_STD:
            patterns.append("Structuring pattern — identical amounts")
        if time_gaps and float(np.mean(time_gaps)) < HIGH_FREQUENCY_HOURS:
            patterns.append("High-frequency transactions")
        if business_type.lower() in SMALL_BUSINESSES and mean_amount > SMALL_BUSINESS_LIMIT:
            patterns.append("Revenue inconsistent with business type")
        if revenue_ratio is not None and revenue_ratio > REVENUE_RATIO_LIMIT:
            patterns.append(f"Laundered volume is {revenue_ratio * 100:.0f}% of declared revenue")

        risk_score = min(100.0, len(patterns) * 20.0 + (40.0 if anomaly else 0.0))

        return {
            "risk_score": round(risk_score, 2),
            "anomaly_detected": anomaly,
            "suspicious_patterns": patterns,
            "recommendation": "Reduce transaction sizes" if risk_score > 50 else "Patterns look normal",
            "revenue_ratio": revenue_ratio,
        }


laundering_detector = LaunderingDetector()
