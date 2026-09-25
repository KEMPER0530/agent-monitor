import json
import os
import time
import uuid
from decimal import Decimal

import boto3
from boto3.dynamodb.conditions import Key


ENDPOINT_URL = os.environ.get("LOCALSTACK_ENDPOINT") or None
DEFAULT_AGENT = os.environ.get("DEFAULT_AGENT", "codex")


# DynamoDBリソースはLambdaの実行環境で再利用し、コールドスタート後の接続作成を減らします。
dynamodb = boto3.resource("dynamodb", endpoint_url=ENDPOINT_URL)


# API GatewayからのHTTPメソッドに応じて登録または参照へ振り分けます。
def handler(event, context):
    method = event.get("httpMethod", "GET")
    if method == "POST":
        return create_event(event)
    if method == "GET":
        return get_snapshot(event)
    return response(405, {"error": "method not allowed"})


# イベント登録はagentに応じてCodex用またはClaude用のテーブルへ保存します。
def create_event(event):
    try:
        payload = json.loads(event.get("body") or "{}")
    except json.JSONDecodeError as exc:
        return response(400, {"error": str(exc)})

    agent_key = resolve_agent(payload.get("agent") or query_param(event, "agent"))
    table, error = table_for_agent(agent_key)
    if error:
        return error

    now_ms = int(time.time() * 1000)
    item = {
        "pk": "EVENT",
        "sk": f"{now_ms}#{payload.get('id') or str(uuid.uuid4())}",
        "id": payload.get("id") or str(uuid.uuid4()),
        "type": payload.get("type", "task"),
        "status": payload.get("status", "info"),
        "title": payload.get("title", ""),
        "message": payload.get("message", ""),
        "agent": agent_key,
        "agentLabel": payload.get("agent", agent_key),
        "taskId": payload.get("taskId", ""),
        "costUsd": Decimal(str(payload.get("costUsd", 0))),
        "tokens": int(payload.get("tokens", 0)),
        "toolCalls": int(payload.get("toolCalls", 0)),
        "createdAt": payload.get("createdAt") or time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    }
    if not item["title"]:
        return response(400, {"error": "title is required"})
    table.put_item(Item=item)
    return response(201, normalize(item))


# ダッシュボードが必要な集計値をagent別のDynamoDBテーブルから作ります。
def get_snapshot(event):
    agent_key = resolve_agent(query_param(event, "agent"))
    table, error = table_for_agent(agent_key)
    if error:
        return error

    result = table.query(KeyConditionExpression=Key("pk").eq("EVENT"), ScanIndexForward=True)
    events = [normalize(item) for item in result.get("Items", [])]
    summary = {
        "agent": agent_key,
        "totalEvents": len(events),
        "runningTasks": sum(1 for item in events if item["type"] == "task" and item["status"] == "running"),
        "failedEvents": sum(1 for item in events if item["status"] == "failed"),
        "openQuestions": sum(1 for item in events if item["type"] == "question" and item["status"] != "success"),
        "totalCostUsd": sum(float(item.get("costUsd", 0)) for item in events),
        "totalTokens": sum(int(item.get("tokens", 0)) for item in events),
        "totalToolCalls": sum(int(item.get("toolCalls", 0)) for item in events),
        "lastEventMessage": events[-1]["message"] or events[-1]["title"] if events else "",
    }
    return response(
        200,
        {
            "generatedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "summary": summary,
            "events": events,
        },
    )


# queryStringParametersはAPI Gatewayの設定によりNoneになるため安全に取り出します。
def query_param(event, name):
    params = event.get("queryStringParameters") or {}
    return params.get(name)


# 保存先agentはcodex/claudeに正規化し、不明な値はデフォルトへ寄せます。
def resolve_agent(agent):
    value = (agent or DEFAULT_AGENT or "codex").lower()
    if "claude" in value:
        return "claude"
    if "codex" in value:
        return "codex"
    return DEFAULT_AGENT if DEFAULT_AGENT in ["codex", "claude"] else "codex"


# agentごとの有効フラグとテーブル名を確認してDynamoDB Tableを返します。
def table_for_agent(agent):
    if agent == "claude":
        if os.environ.get("CLAUDE_MONITOR_ENABLED", "true").lower() != "true":
            return None, response(403, {"error": "claude monitoring is disabled"})
        table_name = os.environ.get("CLAUDE_EVENTS_TABLE_NAME")
    else:
        if os.environ.get("CODEX_MONITOR_ENABLED", "true").lower() != "true":
            return None, response(403, {"error": "codex monitoring is disabled"})
        table_name = os.environ.get("CODEX_EVENTS_TABLE_NAME")

    if not table_name:
        return None, response(500, {"error": f"{agent} table is not configured"})
    return dynamodb.Table(table_name), None


# DecimalなどDynamoDB固有の型をJSONレスポンスで扱える値へ戻します。
def normalize(item):
    return {
        "id": item.get("id", ""),
        "type": item.get("type", ""),
        "status": item.get("status", ""),
        "title": item.get("title", ""),
        "message": item.get("message", ""),
        "agent": item.get("agent", ""),
        "agentLabel": item.get("agentLabel", item.get("agent", "")),
        "taskId": item.get("taskId", ""),
        "costUsd": float(item.get("costUsd", 0)),
        "tokens": int(item.get("tokens", 0)),
        "toolCalls": int(item.get("toolCalls", 0)),
        "createdAt": item.get("createdAt", ""),
    }


# Lambda Proxy Integration向けのレスポンス形式へ揃えます。
def response(status_code, body):
    return {
        "statusCode": status_code,
        "headers": {
            "content-type": "application/json",
            "access-control-allow-origin": "*",
        },
        "body": json.dumps(body),
    }
