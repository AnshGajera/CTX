"""Deterministic chunker mirroring internal/ai/chunk.go."""
from __future__ import annotations

from typing import Any

MAX_CHARS = 2000
OVERLAP = 200


def _split(text: str, max_chars: int = MAX_CHARS, overlap: int = OVERLAP) -> list[str]:
    text = (text or "").strip()
    if not text:
        return []
    if len(text) <= max_chars:
        return [text]
    out = []
    step = max_chars - overlap
    for i in range(0, len(text), step):
        out.append(text[i : i + max_chars])
        if i + max_chars >= len(text):
            break
    return out


def chunk_context(ctx: dict[str, Any]) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []

    def add(kind: str, id_: str, text: str, source: str = ""):
        for part in _split(text):
            out.append({"id": id_, "kind": kind, "text": part, "source": source, "snippet": part[:280]})

    for e in ((ctx.get("apis") or {}).get("endpoints") or []):
        add("api", f"{e.get('method', '')} {e.get('path', '')}",
            f"{e.get('method', '')} {e.get('path', '')} handler={e.get('handler', '')} file={e.get('file', '')}",
            e.get("file", ""))
    for m in ((ctx.get("database") or {}).get("models") or []):
        fields = ", ".join(f"{f.get('name')}:{f.get('type')}" for f in (m.get("fields") or []))
        add("model", f"model:{m.get('name', '')}", f"model {m.get('name', '')} fields: {fields}", m.get("file", ""))
    for v in ((ctx.get("environment") or {}).get("variables") or []):
        add("env", f"env:{v.get('name', '')}", f"env {v.get('name', '')} category={v.get('category', '')} {v.get('description', '')}")
    for k in ((ctx.get("file_structure") or {}).get("key_files") or []):
        add("file", f"file:{k.get('path', '')}", f"key file {k.get('path', '')} purpose={k.get('purpose', '')}", k.get("path", ""))
    arch = ctx.get("architecture") or {}
    if arch:
        add("architecture", "architecture", f"pattern={arch.get('pattern', '')}")
    return out
