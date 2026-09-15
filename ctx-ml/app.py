"""ctx-ml: FastAPI sidecar for embeddings + semantic search + summarization."""
from __future__ import annotations

import math
import os
from collections import Counter
from typing import Any, Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

try:
    from sentence_transformers import SentenceTransformer  # type: ignore
except Exception:  # pragma: no cover - optional dep
    SentenceTransformer = None  # type: ignore

from ranker import tfidf_rank
from chunker import chunk_context
from ast_extract import extract_routes

app = FastAPI(title="ctx-ml", version="1.0.0")

_model = None
_model_name = os.environ.get("CTX_EMBED_MODEL", "all-MiniLM-L6-v2")


def get_model():
    global _model
    if _model is None and SentenceTransformer is not None:
        try:
            _model = SentenceTransformer(_model_name)
        except Exception:
            _model = None
    return _model


class Chunk(BaseModel):
    id: str = ""
    kind: str = ""
    text: str
    source: Optional[str] = ""


class SearchRequest(BaseModel):
    query: str
    chunks: list[Chunk]
    top_k: int = 5


class ChunkRequest(BaseModel):
    context: dict[str, Any]


class SummarizeRequest(BaseModel):
    context: dict[str, Any]
    style: str = "onboarding"


class ExtractRoutesRequest(BaseModel):
    root: str


@app.get("/health")
def health() -> dict[str, Any]:
    return {"status": "ok", "model": _model_name, "loaded": get_model() is not None}


@app.post("/chunk")
def chunk(req: ChunkRequest) -> dict[str, Any]:
    return {"chunks": chunk_context(req.context)}


@app.post("/search")
def search(req: SearchRequest) -> dict[str, Any]:
    if len(req.chunks) > 20000:
        raise HTTPException(status_code=413, detail="too many chunks (max 20000)")
    top_k = max(1, min(int(req.top_k), 50))
    model = get_model()
    texts = [(c.text or "")[:20000] for c in req.chunks]
    if model is not None and texts:
        try:
            import numpy as np  # type: ignore

            q = model.encode([req.query], normalize_embeddings=True)
            d = model.encode(texts, normalize_embeddings=True)
            sims = (d @ q[0]).tolist()
            ranked = sorted(zip(req.chunks, sims), key=lambda x: x[1], reverse=True)[:top_k]
            return {"results": [{"text": (c.text or "")[:20000], "source": c.source or "", "score": float(s)} for c, s in ranked], "backend": "minilm"}
        except Exception:
            pass
    # TF-IDF fallback (also used for offline eval parity with Go)
    truncated = [{**c.model_dump(), "text": (c.text or "")[:20000]} for c in req.chunks]
    ranked = tfidf_rank(req.query, truncated, top_k)
    return {"results": ranked, "backend": "tfidf"}


@app.post("/extract/routes")
def extract_routes_ep(req: ExtractRoutesRequest) -> dict[str, Any]:
    """AST-based route extraction for a local repo root."""
    import os as _os

    if not _os.path.isdir(req.root):
        return {"endpoints": [], "error": "not a directory"}
    return {"endpoints": extract_routes(req.root)}


@app.post("/summarize")
def summarize(req: SummarizeRequest) -> dict[str, Any]:
    """Extractive summary: top chunks per section (no LLM key required)."""
    ctx = req.context
    chunks = chunk_context(ctx)
    profile = ctx.get("profile", {})
    langs = ", ".join(l.get("name", "") for l in profile.get("languages", [])[:4])
    n_ep = len((ctx.get("apis") or {}).get("endpoints", []))
    n_models = len((ctx.get("database") or {}).get("models", []))
    summary = (
        f"Project {ctx.get('project_name', '?')} ({ctx.get('profile', {}).get('project_type', '?')}) "
        f"in {langs} with {n_ep} endpoints and {n_models} models. "
    )
    top = tfidf_rank("architecture auth database api deploy", chunks, 5)
    bullets = [t["text"][:220] for t in top]
    guide = "Onboarding: 1) read key files, 2) check required env vars, 3) run migrations, 4) start API server."
    if req.style == "architecture":
        return {"summary": summary, "bullets": bullets}
    return {"summary": summary, "bullets": bullets, "onboarding": guide}
