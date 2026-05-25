import numpy as np
from sklearn.ensemble import GradientBoostingClassifier
import networkx as nx
from typing import Dict

class DEAAgent:
    def __init__(self):
        self.model = GradientBoostingClassifier(n_estimators=100, random_state=42)
        self._train_mock_model()
        self.evidence_graph = nx.DiGraph()

    def _train_mock_model(self):
        np.random.seed(42)
        X = np.random.rand(500, 6)
        y = (X[:, 0] * 0.4 + X[:, 1] * 0.3 + X[:, 2] * 0.2 + X[:, 3] * 0.1 > 0.5).astype(int)
        self.model.fit(X, y)

    def calculate_threat(self, features: Dict) -> Dict:
        X = np.array([[
            features["heat_level"] / 100,
            features["flagged_transactions"] / 20,
            features["batch_yield"] / 1000,
            features["territory_count"] / 10,
            features["days_active"] / 365,
            np.random.rand() * 0.1
        ]])
        prob = self.model.predict_proba(X)[0][1]
        score = round(prob * 100, 2)

        if score < 25:
            risk, prediction = "low", "Hank suspects nothing. Stay clean."
        elif score < 50:
            risk, prediction = "medium", "Hank is sniffing around. Reduce activity."
        elif score < 75:
            risk, prediction = "high", "A case is being built. Lawyer up."
        else:
            risk, prediction = "critical", "They know. It's over — unless you talk to Saul."

        return {
            "threat_score": score,
            "risk_level": risk,
            "prediction": prediction,
            "recommendation": self._get_recommendation(risk)
        }

    def _get_recommendation(self, risk: str) -> str:
        recs = {
            "low": "Continue operations. Keep front businesses active.",
            "medium": "Pause large transactions. Increase laundering ratio.",
            "high": "Shut down one territory. Consult Saul immediately.",
            "critical": "Full operational freeze. Destroy evidence. Run."
        }
        return recs.get(risk, "Unknown")

    def add_evidence_node(self, empire_id: int, evidence_type: str, weight: float):
        node = f"{empire_id}_{evidence_type}"
        self.evidence_graph.add_node(node, weight=weight)
        if len(self.evidence_graph.nodes) > 1:
            nodes = list(self.evidence_graph.nodes)
            self.evidence_graph.add_edge(nodes[-2], node, weight=weight)

dea_agent = DEAAgent()