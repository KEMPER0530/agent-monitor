import importlib
import json
import os

# タイトルなしイベントを拒否し、DynamoDBへ不完全なデータを保存しないことを確認します。
def test_rejects_event_without_title(monkeypatch):
    monkeypatch.setenv("CODEX_EVENTS_TABLE_NAME", "dummy")
    monkeypatch.setenv("CODEX_MONITOR_ENABLED", "true")
    app = importlib.import_module("app")
    event = {"httpMethod": "POST", "body": json.dumps({"type": "task"})}

    result = app.create_event(event)

    assert result["statusCode"] == 400


# agent=claudeの参照時はClaude用テーブルを選ぶことを確認します。
def test_resolves_claude_table(monkeypatch):
    monkeypatch.setenv("CLAUDE_EVENTS_TABLE_NAME", "claude-table")
    monkeypatch.setenv("CLAUDE_MONITOR_ENABLED", "true")
    app = importlib.import_module("app")

    table, error = app.table_for_agent("claude")

    assert error is None
    assert table.name == "claude-table"
