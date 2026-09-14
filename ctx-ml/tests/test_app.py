from fastapi.testclient import TestClient

from app import app


def test_health():
    c = TestClient(app)
    r = c.get("/health")
    assert r.status_code == 200
    assert r.json()["status"] == "ok"


def test_search_tfidf_fallback():
    c = TestClient(app)
    r = c.post("/search", json={
        "query": "GET /api/users",
        "chunks": [{"id": "1", "kind": "api", "text": "GET /api/users handler=listUsers", "source": "a.ts"}],
        "top_k": 3,
    })
    assert r.status_code == 200
    assert len(r.json()["results"]) == 1


def test_chunk_and_summarize():
    c = TestClient(app)
    ctx = {"project_name": "demo", "profile": {"project_type": "api_service", "languages": [{"name": "Go"}]},
           "apis": {"endpoints": [{"method": "GET", "path": "/health", "handler": "h", "file": "main.go"}]}}
    r = c.post("/chunk", json={"context": ctx})
    assert r.status_code == 200
    assert len(r.json()["chunks"]) >= 1
    r = c.post("/summarize", json={"context": ctx, "style": "architecture"})
    assert r.status_code == 200
    assert "summary" in r.json()
