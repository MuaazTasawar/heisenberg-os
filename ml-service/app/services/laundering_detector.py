import numpy as np
from sklearn.ensemble import IsolationForest
from typing import List

class LaunderingDetector:
    def __init__(self):
        self.model = IsolationForest(contamination=0.15, random_state=42)
        self._fit_baseline()

    def _fit_baseline(self):
        normal_data = np.random.exponential(scale=5000, size=(300, 3))
        self.model.fit(normal_data)

    def analyze(self, amounts: List[float], revenues: List[float], time_gaps: List[float], btype: str) -> dict:
        if not amounts:
            return {"risk_score": 0, "anomaly_detected": False, "suspicious_patterns": [], "recommendation": "No data"}

        features = np.array([[np.mean(amounts), np.std(amounts) if len(amounts) > 1 else 0, np.mean(time_gaps) if time_gaps else 0]])
        pred = self.model.predict(features)[0]
        anomaly = pred == -1

        patterns = []
        if np.max(amounts) > 50000:
            patterns.append("Large single transaction detected")
        if len(amounts) > 1 and np.std(amounts) < 100:
            patterns.append("Structuring pattern — identical amounts")
        if time_gaps and np.mean(time_gaps) < 1:
            patterns.append("High-frequency transactions")
        if btype in ["car_wash", "nail_salon"] and np.mean(amounts) > 20000:
            patterns.append("Revenue inconsistent with business type")

        risk_score = round((len(patterns) * 20) + (40 if anomaly else 0), 2)
        risk_score = min(100, risk_score)

        return {
            "risk_score": risk_score,
            "anomaly_detected": anomaly,
            "suspicious_patterns": patterns,
            "recommendation": "Reduce transaction sizes" if risk_score > 50 else "Patterns look normal"
        }

laundering_detector = LaunderingDetector()