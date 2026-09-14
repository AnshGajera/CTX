"""AST-based route extraction.

Precision complement to the Go regex extractors:
- Python: stdlib `ast` (no extra deps) for Flask / FastAPI / Django.
- TypeScript/JavaScript: tree-sitter when installed, otherwise skipped
  (Go regex extractors remain the fallback).

All functions return lists of endpoint dicts:
{method, path, handler, file, line, framework}
"""
from __future__ import annotations

import ast
import os

SKIP_DIRS = {
    "node_modules", ".git", "vendor", "dist", "build", ".next",
    "__pycache__", ".venv", "venv", ".idea", ".vscode", "coverage",
    ".ctx", ".turbo", ".cache", "target", ".nuxt", ".output", "out",
}

HTTP_METHODS = {"get", "post", "put", "patch", "delete"}

try:  # optional; TS extraction degrades gracefully without it
    from tree_sitter import Language, Parser  # type: ignore
    import tree_sitter_typescript as _tst  # type: ignore
    _TS_AVAILABLE = True
except Exception:  # pragma: no cover
    _TS_AVAILABLE = False


def _iter_files(root: str, exts: set[str]):
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        rel_dir = os.path.relpath(dirpath, root).replace(os.sep, "/")
        if rel_dir.split("/")[0] in {"tests", "test"}:
            continue
        for fn in filenames:
            lower = fn.lower()
            if lower.startswith("test_") or lower.endswith(("_test.py", "_test.ts", "_test.js", ".test.ts", ".spec.ts")):
                continue
            if os.path.splitext(fn)[1].lower() in exts:
                full = os.path.join(dirpath, fn)
                try:
                    if os.path.getsize(full) > 1_000_000:
                        continue
                except OSError:
                    continue
                yield full


def _rel(root: str, path: str) -> str:
    return os.path.relpath(path, root).replace(os.sep, "/")


def _str_const(node: ast.AST) -> str | None:
    if isinstance(node, ast.Constant) and isinstance(node.value, str):
        return node.value
    return None


def extract_python_routes(root: str) -> list[dict]:
    endpoints: list[dict] = []
    for path in _iter_files(root, {".py"}):
        try:
            with open(path, encoding="utf-8", errors="ignore") as f:
                tree = ast.parse(f.read(), filename=path)
        except (SyntaxError, ValueError):
            continue
        rel = _rel(root, path)
        for node in ast.walk(tree):
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
                for dec in node.decorator_list:
                    ep = _flask_fastapi_endpoint(dec, node.name, rel)
                    if ep:
                        endpoints.append(ep)
            elif isinstance(node, ast.Call):
                # Django path()/re_path() live inside lists, not Expr nodes.
                # (Decorator calls like router.get() are Attribute-named
                #  get/post/... and are ignored here by name check.)
                ep = _django_path_call(node, rel)
                if ep:
                    endpoints.append(ep)
    return endpoints


def _flask_fastapi_endpoint(dec: ast.AST, handler: str, rel: str) -> dict | None:
    if not isinstance(dec, ast.Call) or not isinstance(dec.func, ast.Attribute):
        return None
    attr = dec.func.attr
    base = dec.func.value.id if isinstance(dec.func.value, ast.Name) else ""
    framework = "fastapi" if base in {"app", "router", "api"} else "flask"
    method = attr.upper()
    if attr == "route":
        method = "GET"
        for kw in dec.keywords:
            if kw.arg == "methods" and isinstance(kw.value, (ast.List, ast.Tuple)):
                vals = [_str_const(e) for e in kw.value.elts]
                vals = [v.upper() for v in vals if v]
                method = vals[0] if len(vals) == 1 else ",".join(vals)
                break
    elif attr.lower() not in HTTP_METHODS:
        return None
    if not dec.args:
        return None
    route = _str_const(dec.args[0])
    if route is None:
        return None
    return {"method": method, "path": route, "handler": handler,
            "file": rel, "line": dec.lineno, "framework": framework}


def _django_path_call(call: ast.Call, rel: str) -> dict | None:
    func = call.func
    name = func.attr if isinstance(func, ast.Attribute) else (func.id if isinstance(func, ast.Name) else "")
    if name not in {"path", "re_path", "url"} or not call.args:
        return None
    route = _str_const(call.args[0])
    if route is None:
        return None
    handler = ""
    if len(call.args) > 1:
        v = call.args[1]
        handler = v.attr if isinstance(v, ast.Attribute) else (v.id if isinstance(v, ast.Name) else "")
    return {"method": "ALL", "path": "/" + route, "handler": handler,
            "file": rel, "line": call.lineno, "framework": "django"}


def _ts_parser():
    """Build a TypeScript parser across tree-sitter API versions; None if unavailable."""
    if not _TS_AVAILABLE:
        return None
    try:
        for getter in ("language_typescript", "language_tsx", "language"):
            fn = getattr(_tst, getter, None)
            if fn is None:
                continue
            try:
                lang = Language(fn())
            except Exception:
                continue
            parser = Parser()
            try:
                parser.language = lang  # new API (property)
            except Exception:
                try:
                    parser.set_language(lang)  # old API
                except Exception:
                    continue
            return parser
    except Exception:
        return None
    return None


def _ts_string_text(node, src: bytes) -> str | None:
    if node.type in ("string", "string_fragment", "template_string"):
        # find first quoted child or strip manually
        text = src[node.start_byte:node.end_byte].decode("utf-8", "ignore")
        return text.strip().strip("'\"`")
    return None


def extract_ts_routes(root: str) -> list[dict]:
    parser = _ts_parser()
    if parser is None:
        return []
    endpoints: list[dict] = []
    for path in _iter_files(root, {".ts", ".tsx", ".js", ".jsx", ".mjs"}):
        base = os.path.basename(path)
        if base.endswith((".test.ts", ".spec.ts", ".test.js", ".test.tsx")):
            continue
        try:
            with open(path, "rb") as f:
                src = f.read()
            tree = parser.parse(src)
        except Exception:
            continue
        rel = _rel(root, path)
        stack = [tree.root_node]
        while stack:
            node = stack.pop()
            stack.extend(node.children)
            if node.type != "call_expression" or not node.children:
                continue
            func = node.child_by_field_name("function")
            args = node.child_by_field_name("arguments")
            if func is None or args is None or func.type != "member_expression":
                continue
            prop = func.child_by_field_name("property")
            if prop is None or prop.text.decode("utf-8", "ignore").lower() not in HTTP_METHODS | {"all"}:
                continue
            method = prop.text.decode("utf-8", "ignore").upper()
            str_node = next((c for c in args.children if c.type in ("string", "template_string")), None)
            if str_node is None:
                continue
            route = _ts_string_text(str_node, src)
            if not route:
                continue
            # handler: last identifier argument
            handler = ""
            for c in reversed(args.children):
                if c.type == "identifier":
                    handler = c.text.decode("utf-8", "ignore")
                    break
            endpoints.append({"method": method, "path": route, "handler": handler,
                              "file": rel, "line": str_node.start_point[0] + 1,
                              "framework": "express-like"})
    return endpoints


def extract_routes(root: str) -> list[dict]:
    return extract_python_routes(root) + extract_ts_routes(root)
