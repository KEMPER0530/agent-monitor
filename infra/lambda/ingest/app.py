import json
import os
import time
import uuid
from decimal import Decimal

import boto3
from boto3.dynamodb.conditions import Key


TABLE_NAME = os.environ["TABLE_NAME"]
ENDPOINT_URL = os.environ.get("LOCALSTACK_ENDPOINT") or None
dynamodb = boto3.resource("dynamodb", endpoint_url=ENDPOINT_URL)
table = dynamodb.Table(TABLE_NAME)


def handler(event, context):
    method = event.get("httpMethod", "GET")
    if method == "POST":
        return create_event(event)
    if method == "GET":
        return get_snapshot()
    return response(405, {"error": "method not allowed"})


def create_event(event):
    try:
        payload = json.loads(event.get("body") or "{}")
    except json.JSONDecodeError as exc:
        return response(400, {"error": str(exc)})

    now_ms = int(time.time() * 1000)
    item = {
        "pk": "EVENT",
        "sk": f"{now_ms}#{payload.get('id') or str(uuid.uuid4())}",
        "id": payload.get("id") or str(uuid.uuid4()),
        "type": payload.get("type", "task"),
        "status": payload.get("status", "info"),
        "title": payload.get("title", ""),
        "message": payload.get("message", ""),
        "agent": payload.get("agent", ""),
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


def get_snapshot():
    result = table.query(KeyConditionExpression=Key("pk").eq("EVENT"), ScanIndexForward=True)
    events = [normalize(item) for item in result.get("Items", [])]
    summary = {
        "totalEvents": len(events),
        "runningTasks": sum(1 for item in events if item["type"] == "task" and item["status"] == "running"),
        "failedEvents": sum(1 for item in events if item["status"] == "failed"),
        "openQuestions": sum(1 for item in events if item["type"] == "question" and item["status"] != "success"),
        "totalCostUsd": sum(float(item.get("costUsd", 0)) for item in events),
        "totalTokens": sum(int(item.get("tokens", 0)) for item in events),
        "totalToolCalls": sum(int(item.get("toolCalls", 0)) for item in events),
        "lastEventMessage": events[-1]["message"] or events[-1]["title"] if events else "",
    }
    return response(200, {"generatedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "summary": summary, "events": events})


def normalize(item):
    return {
        "id": item.get("id", ""),
        "type": item.get("type", ""),
        "status": item.get("status", ""),
        "title": item.get("title", ""),
        "message": item.get("message", ""),
        "agent": item.get("agent", ""),
        "taskId": item.get("taskId", ""),
        "costUsd": float(item.get("costUsd", 0)),
        "tokens": int(item.get("tokens", 0)),
        "toolCalls": int(item.get("toolCalls", 0)),
        "createdAt": item.get("createdAt", ""),
    }


def response(status_code, body):
    return {
        "statusCode": status_code,
        "headers": {
            "content-type": "application/json",
            "access-control-allow-origin": "*",
        },
        "body": json.dumps(body),
    }

