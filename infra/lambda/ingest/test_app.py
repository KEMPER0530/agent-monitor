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


# 同じtaskIdの作業が成功済みなら、実行中件数へ残さないことを確認します。
def test_latest_task_events_replaces_running_with_success():
    app = importlib.import_module("app")

    latest = app.latest_task_events(
        [
            {"id": "1", "type": "task", "status": "running", "taskId": "task-1"},
            {"id": "2", "type": "task", "status": "success", "taskId": "task-1"},
        ]
    )

    assert latest["task-1"]["status"] == "success"


# 同じtaskIdの質問が成功済みなら、未対応質問として残さないことを確認します。
def test_latest_question_events_replaces_open_with_success():
    app = importlib.import_module("app")

    latest = app.latest_question_events(
        [
            {"id": "1", "type": "question", "status": "running", "taskId": "question-1"},
            {"id": "2", "type": "question", "status": "success", "taskId": "question-1"},
        ]
    )

    assert latest["question-1"]["status"] == "success"
