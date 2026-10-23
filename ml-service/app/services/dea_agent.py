"""DEA threat model and evidence graph.

The classifier is trained at start-up on synthetic data with a fixed seed, so the same
input always produces the same score. Labels are drawn from a smooth probability curve
(not a hard threshold), so scores are graded instead of jumping between 0 and 100.

A logistic regression is used on purpose: the synthetic labels follow a logistic curve, and
unlike a tree ensemble it stays accurate in the corners of the feature space, which is
where real game states live (maximum heat, no territory, brand-new empire). All learned
coefficients are positive, so more heat, flagged transactions, yield, territory or time
active can never *lower* the threat.
"""
import threading
from collections import OrderedDict
from typing import Dict, List

import networkx as nx
import numpy as np
from sklearn.linear_model import LogisticRegression

# heat, flagged transactions, batch yield, territories, days active
FEATURE_WEIGHTS = np.array([0.35, 0.30, 0.15, 0.10, 0.10])
SCALES = np.array([100.0, 20.0, 1000.0, 10.0, 365.0])

RISK_BANDS = (
    (25.0, "low", "Hank suspects nothing. Stay clean.", "Continue operations. Keep front businesses active."),
    (50.0, "medium", "Hank is sniffing around. Reduce activity.", "Pause large transactions. Increase laundering ratio."),
    (75.0, "high", "A case is being built. Lawyer up.", "Shut down one territory. Consult Saul immediately."),
    (float("inf"), "critical", "They know. It's over — unless you talk to Saul.", "Full operational freeze. Destroy evidence. Run."),
)


def risk_band(score: float):
    for upper, level, prediction, recommendation in RISK_BANDS:
        if score < upper:
            return level, prediction, recommendation
    raise AssertionError("unreachable")


class EvidenceStore:
    """Per-empire evidence graphs: one edge from every evidence source to the empire node."""

    MAX_EDGES_PER_EMPIRE = 500
    MAX_EMPIRES = 10_000

    def __init__(self) -> None:
        self._graphs: "OrderedDict[int, nx.DiGraph]" = OrderedDict()
        self._lock = threading.Lock()

    @staticmethod
    def _target(empire_id: int) -> str:
        return f"empire:{empire_id}"

    def add(self, empire_id: int, source: str, weight: float, kind: str = "transaction") -> None:
        target = self._target(empire_id)
        with self._lock:
            graph = self._graphs.get(empire_id)
            if graph is None:
                graph = nx.DiGraph()
                graph.add_node(target, kind="empire")
                self._graphs[empire_id] = graph
                if len(self._graphs) > self.MAX_EMPIRES:
                    self._graphs.popitem(last=False)
            if graph.has_edge(source, target):
                graph[source][target]["weight"] += weight
            else:
                if graph.number_of_edges() >= self.MAX_EDGES_PER_EMPIRE:
                    weakest = min(graph.in_edges(target, data="weight"), key=lambda e: e[2])[0]
                    graph.remove_node(weakest)
                graph.add_node(source, kind=kind)
                graph.add_edge(source, target, weight=weight)
            self._graphs.move_to_end(empire_id)

    def summary(self, empire_id: int, top: int = 5) -> Dict:
        target = self._target(empire_id)
        with self._lock:
            graph = self._graphs.get(empire_id)
            if graph is None:
                return {"empire_id": empire_id, "node_count": 0, "total_weight": 0.0, "top_sources": []}
            edges = [(src, w) for src, _, w in graph.in_edges(target, data="weight")]
            kinds = {src: graph.nodes[src].get("kind", "transaction") for src, _ in edges}
        edges.sort(key=lambda e: (-e[1], e[0]))
        return {
            "empire_id": empire_id,
            "node_count": len(edges),
            "total_weight": round(sum(w for _, w in edges), 2),
            "top_sources": [
                {"source": src, "kind": kinds[src], "weight": round(w, 2)} for src, w in edges[:top]
            ],
        }


class DEAAgent:
    def __init__(self) -> None:
        self.model = LogisticRegression(C=100.0, max_iter=1000)
        self.evidence = EvidenceStore()
        self._train()

    def _train(self) -> None:
        rng = np.random.default_rng(42)
        # Half uniform, half U-shaped (Beta 0.4/0.4) so extreme states are well covered.
        X = np.vstack([rng.random((6000, 5)), rng.beta(0.4, 0.4, (6000, 5))])
        pressure = X @ FEATURE_WEIGHTS
        probability = 1.0 / (1.0 + np.exp(-8.0 * (pressure - 0.5)))
        y = (rng.random(len(X)) < probability).astype(int)
        self.model.fit(X, y)

    @staticmethod
    def _features(data: Dict) -> np.ndarray:
        raw = np.array(
            [
                data["heat_level"],
                data["flagged_transactions"],
                data["batch_yield"],
                data["territory_count"],
                data["days_active"],
            ],
            dtype=float,
        )
        return np.clip(raw / SCALES, 0.0, 1.0).reshape(1, -1)

    def calculate_threat(self, data: Dict) -> Dict:
        probability = float(self.model.predict_proba(self._features(data))[0][1])
        score = round(probability * 100, 2)
        level, prediction, recommendation = risk_band(score)
        return {
            "threat_score": score,
            "risk_level": level,
            "prediction": prediction,
            "recommendation": recommendation,
        }


dea_agent = DEAAgent()
