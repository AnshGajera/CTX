"""TF-IDF cosine ranker (parity with Go internal/ai/rank.go)."""
from __future__ import annotations

import math
import re
from collections import Counter

TOKEN_RE = re.compile(r"[a-z0-9_]+")


def tokenize(s: str) -> list[str]:
    return TOKEN_RE.findall((s or "").lower())


def tfidf_rank(query: str, chunks: list[dict], top_k: int = 5) -> list[dict]:
    toks = tokenize(query)
    if not toks or not chunks:
        return []
    docs = [tokenize(c.get("text", "")) for c in chunks]
    n = len(chunks)
    df: Counter[str] = Counter()
    for ts in docs:
        for t in set(ts):
            df[t] += 1
    q_tf = Counter(toks)
    q_len = len(toks)
    scored = []
    for c, ts in zip(chunks, docs):
        d_tf = Counter(ts)
        d_len = len(ts) or 1
        dot = 0.0
        q_norm = 0.0
        for term, qf in q_tf.items():
            idf = math.log(1 + n / (1 + df.get(term, 0)))
            qw = (qf / q_len) * idf
            dw = (d_tf.get(term, 0) / d_len) * idf
            dot += qw * dw
            q_norm += qw * qw
        d_norm = sum((((v / d_len) * math.log(1 + n / (1 + df.get(t, 0)))) ** 2) for t, v in d_tf.items())
        score = dot / (math.sqrt(q_norm) * math.sqrt(d_norm)) if q_norm and d_norm else 0.0
        if c.get("kind", "").lower() in query.lower():
            score += 0.05
        scored.append({"text": c.get("text", ""), "source": c.get("source", ""), "score": score})
    scored.sort(key=lambda x: x["score"], reverse=True)
    return scored[:top_k] if top_k else scored
