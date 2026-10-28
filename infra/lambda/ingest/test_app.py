import importlib
import json
import os
from unittest.mock import Mock


# 本文や偽装ヘッダーを信用せず、CloudFrontで付与したIPだけを検証して採用します。
def test_source_ip_trust_boundary(monkeypatch):
    app = importlib.import_module("app")
    monkeypatch.setenv("CLOUDFRONT_ORIGIN_SECRET", "test-secret")
    event = {"requestContext": {"identity": {"sourceIp": "192.0.2.1"}},
             "headers": {"X-Agent-Source-Ip": "2001:db8::1", "X-Forwarded-For": "198.51.100.9"}}
    assert app.source_ip(event) == "192.0.2.1"
    event["headers"]["X-Agent-Origin-Secret"] = "test-secret"
    assert app.source_ip(event) == "2001:db8::1"
    event["headers"]["X-Agent-Source-Ip"] = "invalid"
    assert app.source_ip(event) == ""


# 登録からスナップショットまでIPと個別削除キーが保持されることを確認します。
def test_ip_persisted_and_returned(monkeypatch):
    app = importlib.import_module("app")
    table = Mock()
    monkeypatch.setattr(app, "table_for_agent", lambda agent: (table, None))
    result = app.create_event({"body": json.dumps({"title": "example", "sourceIp": "forged"}),
                               "requestContext": {"identity": {"sourceIp": "192.0.2.3"}}})
    assert result["statusCode"] == 201
    item = table.put_item.call_args.kwargs["Item"]
    assert item["sourceIp"] == "192.0.2.3"
    table.query.return_value = {"Items": [item]}
    event = json.loads(app.get_snapshot({})["body"])["events"][0]
    assert event["sourceIp"] == "192.0.2.3"
    assert event["eventKey"] == item["sk"]


# APIキーだけの削除を拒否し、Cognito認証後に指定agentの1件だけを削除します。
def test_delete_requires_auth_and_exact_key(monkeypatch):
    app = importlib.import_module("app")
    monkeypatch.setattr(app, "ENDPOINT_URL", None)
    table = Mock()
    monkeypatch.setattr(app, "table_for_agent", lambda agent: (table, None))
    event = {"httpMethod": "DELETE", "queryStringParameters": {"agent": "claude", "eventKey": "123#id"}}
    assert app.handler(event, None)["statusCode"] == 403
    table.delete_item.assert_not_called()
    event["requestContext"] = {"authorizer": {"claims": {"sub": "user"}}}
    assert app.handler(event, None)["statusCode"] == 200
    table.delete_item.assert_called_once_with(Key={"pk": "EVENT", "sk": "123#id"})
    event["queryStringParameters"]["agent"] = "unknown"
    assert app.handler(event, None)["statusCode"] == 400

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


# ツール詳細はCSV文字列でも配列でも同じ表示用配列に整えます。
def test_normalize_tool_details_removes_blanks_and_duplicates():
    app = importlib.import_module("app")

    assert app.normalize_tool_details("rg, go test, rg, ") == ["rg", "go test"]
    assert app.normalize_tool_details(["curl", "curl", "npm run build"]) == ["curl", "npm run build"]
