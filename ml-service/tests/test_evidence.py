import itertools

import pytest

from app.services.dea_agent import EvidenceStore

_ids = itertools.count(910_000)


@pytest.fixture
def empire_id() -> int:
    """A fresh empire id per test (the store is process-wide)."""
    return next(_ids)


def add(client, empire_id, source, weight=1.0, **extra):
    return client.post("/ml/evidence", json={"empire_id": empire_id, "source": source,
                                             "weight": weight, **extra})


def test_unknown_empire_has_an_empty_file(client, empire_id):
    body = client.get(f"/ml/evidence/{empire_id}").json()
    assert body == {"empire_id": empire_id, "node_count": 0, "total_weight": 0.0, "top_sources": []}


def test_adding_evidence_returns_the_updated_summary(client, empire_id):
    response = add(client, empire_id, "tx:1", 2.5)
    assert response.status_code == 201
    body = response.json()
    assert body["node_count"] == 1
    assert body["total_weight"] == 2.5
    assert body["top_sources"] == [{"source": "tx:1", "kind": "transaction", "weight": 2.5}]


def test_the_same_source_accumulates_weight(client, empire_id):
    add(client, empire_id, "tx:1", 1.0)
    body = add(client, empire_id, "tx:1", 2.0).json()
    assert body["node_count"] == 1
    assert body["total_weight"] == 3.0


def test_strongest_sources_come_first(client, empire_id):
    for source, weight in [("a", 1), ("b", 5), ("c", 3), ("d", 3)]:
        add(client, empire_id, source, weight)
    names = [s["source"] for s in client.get(f"/ml/evidence/{empire_id}").json()["top_sources"]]
    assert names == ["b", "c", "d", "a"]


def test_only_the_top_five_are_listed(client, empire_id):
    for i in range(8):
        add(client, empire_id, f"tx:{i}", i + 1)
    body = client.get(f"/ml/evidence/{empire_id}").json()
    assert body["node_count"] == 8
    assert len(body["top_sources"]) == 5
    assert body["top_sources"][0]["source"] == "tx:7"


def test_empires_do_not_share_evidence(client, empire_id):
    add(client, empire_id, "tx:1", 4)
    other = empire_id + 500_000
    assert client.get(f"/ml/evidence/{other}").json()["node_count"] == 0


def test_kind_is_remembered(client, empire_id):
    add(client, empire_id, "wiretap:7", 2, kind="wiretap")
    assert client.get(f"/ml/evidence/{empire_id}").json()["top_sources"][0]["kind"] == "wiretap"


@pytest.mark.parametrize(
    "payload",
    [
        {"empire_id": 1, "source": "", "weight": 1},
        {"empire_id": 1, "source": "x" * 81, "weight": 1},
        {"empire_id": 1, "source": "tx", "weight": 0},
        {"empire_id": 1, "source": "tx", "weight": -2},
        {"empire_id": 1, "source": "tx", "weight": 5000},
        {"empire_id": -1, "source": "tx", "weight": 1},
        {"source": "tx", "weight": 1},
    ],
)
def test_invalid_evidence_is_rejected(client, payload):
    assert client.post("/ml/evidence", json=payload).status_code == 422


def test_negative_empire_id_in_url_is_rejected(client):
    assert client.get("/ml/evidence/-4").status_code == 422


def test_weakest_source_is_dropped_when_the_graph_is_full(monkeypatch):
    monkeypatch.setattr(EvidenceStore, "MAX_EDGES_PER_EMPIRE", 3)
    store = EvidenceStore()
    for source, weight in [("a", 5), ("b", 1), ("c", 3)]:
        store.add(1, source, weight)
    store.add(1, "d", 4)
    summary = store.summary(1, top=10)
    assert summary["node_count"] == 3
    assert {s["source"] for s in summary["top_sources"]} == {"a", "c", "d"}


def test_oldest_empire_is_dropped_when_the_store_is_full(monkeypatch):
    monkeypatch.setattr(EvidenceStore, "MAX_EMPIRES", 2)
    store = EvidenceStore()
    store.add(1, "x", 1)
    store.add(2, "x", 1)
    store.add(3, "x", 1)
    assert store.summary(1)["node_count"] == 0
    assert store.summary(2)["node_count"] == 1
    assert store.summary(3)["node_count"] == 1
