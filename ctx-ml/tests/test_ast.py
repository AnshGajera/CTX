import os
import tempfile

from ast_extract import extract_python_routes


def _write(tmp, name, content):
    p = os.path.join(tmp, name)
    with open(p, "w") as f:
        f.write(content)
    return p


def test_fastapi_decorators():
    with tempfile.TemporaryDirectory() as tmp:
        _write(tmp, "api.py", """
from fastapi import APIRouter
router = APIRouter()

@router.get('/users')
def list_users():
    return []

@router.post('/users/{uid}', status_code=201)
async def create_user(uid: str):
    return {}
""")
        eps = extract_python_routes(tmp)
        by_path = {(e["method"], e["path"]): e for e in eps}
        assert ("GET", "/users") in by_path
        assert by_path[("GET", "/users")]["handler"] == "list_users"
        assert ("POST", "/users/{uid}") in by_path


def test_flask_methods():
    with tempfile.TemporaryDirectory() as tmp:
        _write(tmp, "app.py", """
from flask import Flask
app = Flask(__name__)

@app.route('/login', methods=['GET', 'POST'])
def login():
    return 'ok'

@app.route('/health')
def health():
    return 'ok'
""")
        eps = extract_python_routes(tmp)
        by_path = {(e["method"], e["path"]): e for e in eps}
        assert ("GET,POST", "/login") in by_path or ("GET", "/login") in by_path
        assert ("GET", "/health") in by_path


def test_django_path():
    with tempfile.TemporaryDirectory() as tmp:
        _write(tmp, "urls.py", """
from django.urls import path
from . import views

urlpatterns = [
    path('articles/', views.article_list),
]
""")
        eps = extract_python_routes(tmp)
        assert any(e["path"] == "/articles/" and e["handler"] == "article_list" for e in eps)


def test_syntax_error_skipped():
    with tempfile.TemporaryDirectory() as tmp:
        _write(tmp, "broken.py", "def foo(:\n")
        assert extract_python_routes(tmp) == []
