import importlib
import json
import os


def test_rejects_event_without_title(monkeypatch):
    monkeypatch.setenv("TABLE_NAME", "dummy")
    app = importlib.import_module("app")
    event = {"httpMethod": "POST", "body": json.dumps({"type": "task"})}

    result = app.create_event(event)

    assert result["statusCode"] == 400

