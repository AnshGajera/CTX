"""Retrieval eval: hit@k over self-derived cases."""
from __future__ import annotations

import json
import sys

from chunker import chunk_context
from ranker import tfidf_rank


def evaluate(context_path: str, k: int = 5) -> dict:
    with open(context_path) as f:
        ctx = json.load(f)
    chunks = chunk_context(ctx)
    cases: list[tuple[str, str]] = []
    for e in ((ctx.get("apis") or {}).get("endpoints") or [])[:5]:
        cases.append((f"{e.get('method', '')} {e.get('path', '')}", e.get("path", "")))
    for m in ((ctx.get("database") or {}).get("models") or [])[:5]:
        cases.append((f"model {m.get('name', '')}", m.get("name", "")))
    if not cases:
        return {"hit_at_k": 0.0, "hits": 0, "total": 0, "note": "no cases"}
    hits = 0
    for query, expect in cases:
        ranked = tfidf_rank(query, chunks, k)
        if any(expect.lower() in r["text"].lower() for r in ranked if expect):
            hits += 1
    return {"hit_at_k": hits / len(cases), "hits": hits, "total": len(cases), "k": k}


if __name__ == "__main__":
    path = sys.argv[1] if len(sys.argv) > 1 else ".ctx/context.json"
    k = int(sys.argv[2]) if len(sys.argv) > 2 else 5
    print(json.dumps(evaluate(path, k), indent=2))
