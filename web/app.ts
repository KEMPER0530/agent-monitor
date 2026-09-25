type EventStatus = "info" | "running" | "success" | "failed" | "blocked";

// MonitorEvent はAPIから受け取る1件分の監視イベントです。
type MonitorEvent = {
  id: string;
  type: string;
  status: EventStatus;
  title: string;
  message?: string;
  agent?: string;
  taskId?: string;
  costUsd?: number;
  tokens?: number;
  toolCalls?: number;
  createdAt: string;
};

// Snapshot は画面全体を描画するための集計済みレスポンスです。
type Snapshot = {
  generatedAt: string;
  summary: {
    totalEvents: number;
    runningTasks: number;
    failedEvents: number;
    openQuestions: number;
    totalCostUsd: number;
    totalTokens: number;
    totalToolCalls: number;
    lastEventMessage?: string;
  };
  events: MonitorEvent[];
};

type AuthConfig = {
  clientId: string;
  domain: string;
  redirectUri: string;
};

declare global {
  interface Window {
    AGENT_MONITOR_AUTH?: AuthConfig;
  }
}

let currentFilter = "all";
let currentAgent = "codex";
let currentSnapshot: Snapshot | null = null;

// byId はDOM取得失敗を早めに検知して、描画崩れの原因を追いやすくします。
const byId = <T extends HTMLElement>(id: string): T => {
  const element = document.getElementById(id);
  if (!element) throw new Error(`missing #${id}`);
  return element as T;
};

// loadSnapshot はサーバーの最新状態を取得し、成功時だけ画面を更新します。
async function loadSnapshot(): Promise<void> {
  const response = await fetch(`/api/snapshot?agent=${encodeURIComponent(currentAgent)}`, {
    cache: "no-store",
    headers: authHeaders(),
  });
  if (!response.ok) throw new Error(`snapshot failed: ${response.status}`);
  currentSnapshot = await response.json();
  render(currentSnapshot);
}

// ensureAuthenticated はS3公開時だけCognito Hosted UIへ誘導します。
function ensureAuthenticated(): boolean {
  const config = window.AGENT_MONITOR_AUTH;
  if (!config) {
    byId<HTMLButtonElement>("logout").hidden = true;
    return true;
  }

  persistTokensFromHash();
  if (sessionStorage.getItem("agentMonitorIdToken")) return true;

  const loginUrl = new URL(`${config.domain}/login`);
  loginUrl.searchParams.set("client_id", config.clientId);
  loginUrl.searchParams.set("response_type", "token");
  loginUrl.searchParams.set("scope", "openid email profile");
  loginUrl.searchParams.set("redirect_uri", config.redirectUri);
  window.location.assign(loginUrl.toString());
  return false;
}

// persistTokensFromHash はCognitoのimplicit flowで返るJWTをセッション内に保存します。
function persistTokensFromHash(): void {
  if (!window.location.hash.includes("id_token")) return;
  const params = new URLSearchParams(window.location.hash.slice(1));
  const idToken = params.get("id_token");
  const accessToken = params.get("access_token");
  if (idToken) sessionStorage.setItem("agentMonitorIdToken", idToken);
  if (accessToken) sessionStorage.setItem("agentMonitorAccessToken", accessToken);
  history.replaceState(null, document.title, window.location.pathname + window.location.search);
}

// authHeaders はAPI Gateway Cognito AuthorizerへIDトークンを渡します。
function authHeaders(): HeadersInit {
  const idToken = sessionStorage.getItem("agentMonitorIdToken");
  return idToken ? { Authorization: `Bearer ${idToken}` } : {};
}

// logout はセッションを消し、Cognito Hosted UIのログアウトURLへ移動します。
function logout(): void {
  const config = window.AGENT_MONITOR_AUTH;
  sessionStorage.removeItem("agentMonitorIdToken");
  sessionStorage.removeItem("agentMonitorAccessToken");
  if (!config) return;
  const logoutUrl = new URL(`${config.domain}/logout`);
  logoutUrl.searchParams.set("client_id", config.clientId);
  logoutUrl.searchParams.set("logout_uri", config.redirectUri);
  window.location.assign(logoutUrl.toString());
}

// render は数値カード、セッション状態、タイムラインをまとめて更新します。
function render(snapshot: Snapshot): void {
  byId("metric-events").textContent = String(snapshot.summary.totalEvents);
  byId("metric-running").textContent = String(snapshot.summary.runningTasks);
  byId("metric-failed").textContent = String(snapshot.summary.failedEvents);
  byId("metric-questions").textContent = String(snapshot.summary.openQuestions);
  byId("metric-cost").textContent = `$${snapshot.summary.totalCostUsd.toFixed(4)}`;
  byId("metric-tokens").textContent = snapshot.summary.totalTokens.toLocaleString();
  byId("metric-tools").textContent = snapshot.summary.totalToolCalls.toLocaleString();
  byId("last-message").textContent = snapshot.summary.lastEventMessage || "まだイベントはありません。";
  byId("last-updated").textContent = new Date(snapshot.generatedAt).toLocaleTimeString();
  byId("event-note").textContent = snapshot.summary.totalEvents === 1 ? "1件のシグナル" : `${snapshot.summary.totalEvents}件のシグナル`;

  const state = deriveSessionState(snapshot);
  byId("session-state").textContent = state.label;
  byId("health-dot").className = `health-dot ${state.className}`;

  renderEvents(snapshot.events);
}

// deriveSessionState は重要度の高い状態から順にサイドバー表示へ変換します。
function deriveSessionState(snapshot: Snapshot): { label: string; className: string } {
  if (snapshot.summary.failedEvents > 0 || snapshot.summary.openQuestions > 0) {
    return { label: "確認が必要", className: "alert" };
  }
  if (snapshot.summary.runningTasks > 0) {
    return { label: "実行中", className: "live" };
  }
  if (snapshot.summary.totalEvents > 0) {
    return { label: "安定", className: "stable" };
  }
  return { label: "待機中", className: "idle" };
}

// renderEvents は選択中フィルタを反映してタイムラインを再描画します。
function renderEvents(events: MonitorEvent[]): void {
  const container = byId("events");
  container.innerHTML = "";

  const filtered = [...events]
    .reverse()
    .filter((event) => currentFilter === "all" || event.status === currentFilter);

  if (filtered.length === 0) {
    container.append(emptyState());
    return;
  }

  for (const event of filtered) {
    container.append(eventRow(event));
  }
}

// emptyState はイベントがない場合もレイアウトを保つための表示です。
function emptyState(): HTMLElement {
  const element = document.createElement("p");
  element.className = "empty";
  element.textContent = "該当するイベントはありません。";
  return element;
}

// eventRow は1件のイベントを状態色付きの行として構築します。
function eventRow(event: MonitorEvent): HTMLElement {
  const article = document.createElement("article");
  article.className = `event ${event.status}`;

  const rail = document.createElement("div");
  rail.className = "event-rail";

  const body = document.createElement("div");
  body.className = "event-body";

  const head = document.createElement("div");
  head.className = "event-head";

  const title = document.createElement("h4");
  title.textContent = event.title;

  const badge = document.createElement("span");
  badge.className = "status-badge";
  badge.textContent = statusLabel(event.status);

  head.append(title, badge);

  const meta = document.createElement("p");
  meta.className = "event-meta";
  meta.textContent = [
    eventTypeLabel(event.type),
    event.agent || "agent",
    new Date(event.createdAt).toLocaleString(),
  ].join(" / ");

  body.append(head, meta);
  if (event.message) {
    const message = document.createElement("p");
    message.className = "message";
    message.textContent = event.message;
    body.append(message);
  }

  article.append(rail, body);
  return article;
}

// statusLabel は保存値を画面表示用の日本語へ変換します。
function statusLabel(status: EventStatus): string {
  const labels: Record<EventStatus, string> = {
    info: "情報",
    running: "実行中",
    success: "成功",
    failed: "失敗",
    blocked: "停止中",
  };
  return labels[status] || status;
}

// eventTypeLabel はイベント種別を日本語表示へ変換します。
function eventTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    task: "タスク",
    tool: "ツール",
    test: "テスト",
    question: "質問",
    error: "エラー",
  };
  return labels[type] || type;
}

// フィルタ変更時は保存済みスナップショットを使い、再通信なしで表示だけ切り替えます。
document.querySelectorAll<HTMLButtonElement>(".filter").forEach((button) => {
  button.addEventListener("click", () => {
    currentFilter = button.dataset.filter || "all";
    document.querySelectorAll(".filter").forEach((item) => item.classList.remove("active"));
    button.classList.add("active");
    if (currentSnapshot) renderEvents(currentSnapshot.events);
  });
});

// agent切替はAWS側のCodex/Claude別DynamoDBテーブルを切り替える操作です。
document.querySelectorAll<HTMLButtonElement>(".agent-filter").forEach((button) => {
  button.addEventListener("click", () => {
    currentAgent = button.dataset.agent || "codex";
    document.querySelectorAll(".agent-filter").forEach((item) => item.classList.remove("active"));
    button.classList.add("active");
    void loadSnapshot();
  });
});

// 手動更新と自動更新の両方で同じ取得処理を使います。
byId<HTMLButtonElement>("refresh").addEventListener("click", () => {
  void loadSnapshot();
});

byId<HTMLButtonElement>("logout").addEventListener("click", logout);

if (ensureAuthenticated()) {
  void loadSnapshot();
  setInterval(() => void loadSnapshot(), 5000);
}

export {};
