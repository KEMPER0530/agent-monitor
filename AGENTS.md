# agent-monitor 運用ルール

このリポジトリでCodexが作業する場合、手動ではなくCodex自身が `agent-monitor` へ進捗イベントを送信します。

## 自動送信タイミング

1. ユーザーから新しい依頼を受けた直後に、`status=running` の `task` イベントを送信します。
2. テスト、ビルド、デプロイ、調査で失敗した場合は、原因が分かった時点で `status=failed` または `status=blocked` のイベントを送信します。
3. 最終回答を返す直前に、結果に応じて `status=success`、`status=failed`、`status=blocked` のいずれかを必ず送信します。
4. ユーザーに手動で `curl` やCLI実行を依頼せず、Codexが実行します。

## Codex用ローカル設定

このプロジェクトではCodex監視を前提にします。

```bash
export AGENT_MONITOR_ENABLED=true
export AGENT_MONITOR_AGENT=codex
export AGENT_MONITOR_API_URL=https://s3-agent-monitor.kemper0530.com
export AGENT_MONITOR_ID_TOKEN=<Cognitoのid_token>
```

`.agent-monitor.env` が存在する場合、`cmd/agent-monitor` は起動時に自動で読み込みます。`.agent-monitor` が存在する場合も監視は有効です。

## AWS APIへ送信する設定

AWS APIへ送る場合は、次の環境変数を使います。

```bash
export AGENT_MONITOR_API_URL=https://s3-agent-monitor.kemper0530.com
export AGENT_MONITOR_ID_TOKEN=<Cognitoのid_token>
```

`AGENT_MONITOR_API_URL` が未設定の場合、または `AGENT_MONITOR_ID_TOKEN` が未設定の場合は、ローカルの `.agent-monitor-data/events.jsonl` に保存します。

## 実行コマンド

開始時:

```bash
AGENT_MONITOR_ENABLED=true AGENT_MONITOR_AGENT=codex go run ./cmd/agent-monitor --type task --status running --title "<作業名>" --agent codex --message "<短い状況>"
```

終了直前:

```bash
AGENT_MONITOR_ENABLED=true AGENT_MONITOR_AGENT=codex go run ./cmd/agent-monitor --type task --status success --title "<作業名>" --agent codex --message "<完了内容>"
```

失敗時:

```bash
AGENT_MONITOR_ENABLED=true AGENT_MONITOR_AGENT=codex go run ./cmd/agent-monitor --type error --status failed --title "<作業名>" --agent codex --message "<原因と対応>"
```

## 失敗時の扱い

監視イベント送信に失敗しても、ユーザー依頼そのものの作業は止めません。ただし、最終回答で監視送信に失敗したことと原因を短く伝えます。
