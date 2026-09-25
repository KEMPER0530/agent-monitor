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
  region: string;
};

declare global {
  interface Window {
    AGENT_MONITOR_AUTH?: AuthConfig;
  }
}

let currentFilter = "all";
let currentAgent = "codex";
let currentSnapshot: Snapshot | null = null;
let refreshTimer: number | undefined;

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

// ensureAuthenticated はS3公開時だけカスタムログイン画面を表示します。
function ensureAuthenticated(): boolean {
  const config = window.AGENT_MONITOR_AUTH;
  if (!config) {
    showDashboard();
    byId<HTMLButtonElement>("logout").hidden = true;
    return true;
  }

  if (sessionStorage.getItem("agentMonitorIdToken")) {
    showDashboard();
    return true;
  }

  showLogin();
  return false;
}

// authHeaders はAPI Gateway Cognito AuthorizerへIDトークンを渡します。
function authHeaders(): HeadersInit {
  const idToken = sessionStorage.getItem("agentMonitorIdToken");
  return idToken ? { Authorization: `Bearer ${idToken}` } : {};
}

// login はCognitoの公開App Clientへ直接認証し、パスワードを保存せずJWTだけ保持します。
async function login(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  const config = window.AGENT_MONITOR_AUTH;
  if (!config) return;

  const username = byId<HTMLInputElement>("login-username").value.trim();
  const password = byId<HTMLInputElement>("login-password").value;
  const submit = byId<HTMLButtonElement>("login-submit");
  const error = byId("login-error");
  error.textContent = "";
  submit.disabled = true;

  try {
    const response = await fetch(`https://cognito-idp.${config.region}.amazonaws.com/`, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-amz-json-1.1",
        "X-Amz-Target": "AWSCognitoIdentityProviderService.InitiateAuth",
      },
      body: JSON.stringify({
        AuthFlow: "USER_PASSWORD_AUTH",
        ClientId: config.clientId,
        AuthParameters: {
          USERNAME: username,
          PASSWORD: password,
        },
      }),
    });
    const body = await response.json();
    if (!response.ok) {
      throw new Error(body.message || body.__type || "ログインに失敗しました。");
    }
    if (body.ChallengeName) {
      throw new Error("追加認証が必要です。Cognito側で初期パスワード変更を完了してください。");
    }
    const result = body.AuthenticationResult;
    if (!result?.IdToken) {
      throw new Error("IDトークンを取得できませんでした。");
    }
    sessionStorage.setItem("agentMonitorIdToken", result.IdToken);
    if (result.AccessToken) sessionStorage.setItem("agentMonitorAccessToken", result.AccessToken);
    if (result.RefreshToken) sessionStorage.setItem("agentMonitorRefreshToken", result.RefreshToken);
    byId<HTMLFormElement>("login-form").reset();
    showDashboard();
    startDashboard();
  } catch (caught) {
    error.textContent = caught instanceof Error ? caught.message : "ログインに失敗しました。";
  } finally {
    submit.disabled = false;
  }
}

// showLogin は未認証時にダッシュボードを隠してログイン画面だけを表示します。
function showLogin(): void {
  byId("login-screen").hidden = false;
  byId("app-frame").hidden = true;
}

// showDashboard は認証後にログイン画面を隠して監視画面を表示します。
function showDashboard(): void {
  byId("login-screen").hidden = true;
  byId("app-frame").hidden = false;
}

// logout はセッションを消し、画面内ログインへ戻します。
function logout(): void {
  const config = window.AGENT_MONITOR_AUTH;
  sessionStorage.removeItem("agentMonitorIdToken");
  sessionStorage.removeItem("agentMonitorAccessToken");
  sessionStorage.removeItem("agentMonitorRefreshToken");
  if (!config) return;
  if (refreshTimer) window.clearInterval(refreshTimer);
  showLogin();
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

// showHelp はカードの中で説明が切れないよう、共通の吹き出しを画面上に配置します。
function showHelp(target: HTMLElement): void {
  const tooltip = byId<HTMLDivElement>("tooltip-layer");
  const message = target.dataset.help;
  if (!message) return;

  tooltip.textContent = message;
  tooltip.hidden = false;

  const targetRect = target.getBoundingClientRect();
  const tooltipRect = tooltip.getBoundingClientRect();
  const margin = 16;
  const centeredLeft = targetRect.left + targetRect.width / 2 - tooltipRect.width / 2;
  const left = Math.min(Math.max(margin, centeredLeft), window.innerWidth - tooltipRect.width - margin);
  const belowTop = targetRect.bottom + 10;
  const aboveTop = targetRect.top - tooltipRect.height - 10;
  const top = belowTop + tooltipRect.height <= window.innerHeight - margin ? belowTop : Math.max(margin, aboveTop);

  tooltip.style.left = `${left}px`;
  tooltip.style.top = `${top}px`;
}

// hideHelp はhover/focusが外れた時に説明を閉じます。
function hideHelp(): void {
  byId("tooltip-layer").hidden = true;
}

// 初心者向け説明はマウス操作とキーボード操作の両方で読めるようにします。
document.querySelectorAll<HTMLElement>("[data-help]").forEach((button) => {
  button.addEventListener("mouseenter", () => showHelp(button));
  button.addEventListener("focus", () => showHelp(button));
  button.addEventListener("mouseleave", hideHelp);
  button.addEventListener("blur", hideHelp);
});

window.addEventListener("scroll", hideHelp, { passive: true });
window.addEventListener("resize", hideHelp);

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
byId<HTMLFormElement>("login-form").addEventListener("submit", (event) => {
  void login(event);
});

// startDashboard はログイン直後と初期表示で同じ監視ループを開始します。
function startDashboard(): void {
  if (refreshTimer) window.clearInterval(refreshTimer);
  void loadSnapshot();
  refreshTimer = window.setInterval(() => void loadSnapshot(), 5000);
}

if (ensureAuthenticated()) {
  startDashboard();
}

export {};
