let currentFilter = "all";
let currentAgent = "codex";
let currentSnapshot = null;
let refreshTimer;
// byId はDOM取得失敗を早めに検知して、描画崩れの原因を追いやすくします。
const byId = (id) => {
    const element = document.getElementById(id);
    if (!element)
        throw new Error(`missing #${id}`);
    return element;
};
// loadSnapshot はサーバーの最新状態を取得し、成功時だけ画面を更新します。
async function loadSnapshot() {
    const response = await fetch(`/api/snapshot?agent=${encodeURIComponent(currentAgent)}`, {
        cache: "no-store",
        headers: authHeaders(),
    });
    if (!response.ok)
        throw new Error(`snapshot failed: ${response.status}`);
    currentSnapshot = await response.json();
    render(currentSnapshot);
}
// ensureAuthenticated はS3公開時だけカスタムログイン画面を表示します。
function ensureAuthenticated() {
    const config = window.AGENT_MONITOR_AUTH;
    if (!config) {
        showDashboard();
        byId("logout").hidden = true;
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
function authHeaders() {
    const idToken = sessionStorage.getItem("agentMonitorIdToken");
    return idToken ? { Authorization: `Bearer ${idToken}` } : {};
}
// login はCognitoの公開App Clientへ直接認証し、パスワードを保存せずJWTだけ保持します。
async function login(event) {
    event.preventDefault();
    const config = window.AGENT_MONITOR_AUTH;
    if (!config)
        return;
    const username = byId("login-username").value.trim();
    const password = byId("login-password").value;
    const submit = byId("login-submit");
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
        if (result.AccessToken)
            sessionStorage.setItem("agentMonitorAccessToken", result.AccessToken);
        if (result.RefreshToken)
            sessionStorage.setItem("agentMonitorRefreshToken", result.RefreshToken);
        byId("login-form").reset();
        showDashboard();
        startDashboard();
    }
    catch (caught) {
        error.textContent = caught instanceof Error ? caught.message : "ログインに失敗しました。";
    }
    finally {
        submit.disabled = false;
    }
}
// showLogin は未認証時にダッシュボードを隠してログイン画面だけを表示します。
function showLogin() {
    byId("login-screen").hidden = false;
    byId("app-frame").hidden = true;
}
// showDashboard は認証後にログイン画面を隠して監視画面を表示します。
function showDashboard() {
    byId("login-screen").hidden = true;
    byId("app-frame").hidden = false;
}
// logout はセッションを消し、画面内ログインへ戻します。
function logout() {
    const config = window.AGENT_MONITOR_AUTH;
    sessionStorage.removeItem("agentMonitorIdToken");
    sessionStorage.removeItem("agentMonitorAccessToken");
    sessionStorage.removeItem("agentMonitorRefreshToken");
    if (!config)
        return;
    if (refreshTimer)
        window.clearInterval(refreshTimer);
    showLogin();
}
// render は数値カード、セッション状態、タイムラインをまとめて更新します。
function render(snapshot) {
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
function deriveSessionState(snapshot) {
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
function renderEvents(events) {
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
function emptyState() {
    const element = document.createElement("p");
    element.className = "empty";
    element.textContent = "該当するイベントはありません。";
    return element;
}
// eventRow は1件のイベントを状態色付きの行として構築します。
function eventRow(event) {
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
    body.append(eventUsage(event));
    article.append(rail, body);
    return article;
}
// eventUsage は履歴ごとの利用量とツール実行詳細を読みやすくまとめます。
function eventUsage(event) {
    const container = document.createElement("div");
    container.className = "event-usage";
    container.append(usageChip("コスト", `$${(event.costUsd ?? 0).toFixed(4)}`), usageChip("トークン", (event.tokens ?? 0).toLocaleString()), usageChip("ツール", (event.toolCalls ?? 0).toLocaleString()));
    const toolDetails = normalizedToolDetails(event);
    if ((event.toolCalls ?? 0) > 0 || toolDetails.length > 0) {
        const details = document.createElement("details");
        details.className = "tool-details";
        const summary = document.createElement("summary");
        summary.textContent = "ツール詳細";
        details.append(summary);
        if (toolDetails.length === 0) {
            const empty = document.createElement("p");
            empty.textContent = "ツール名は未送信です。";
            details.append(empty);
        }
        else {
            const list = document.createElement("ul");
            for (const tool of toolDetails) {
                const item = document.createElement("li");
                item.textContent = tool;
                list.append(item);
            }
            details.append(list);
        }
        container.append(details);
    }
    return container;
}
// usageChip は数値を同じ見た目の小さな指標として返します。
function usageChip(label, value) {
    const chip = document.createElement("span");
    chip.className = "usage-chip";
    const labelElement = document.createElement("span");
    labelElement.textContent = label;
    const valueElement = document.createElement("strong");
    valueElement.textContent = value;
    chip.append(labelElement, valueElement);
    return chip;
}
// normalizedToolDetails はAPIの古い応答にも耐えるため、表示前に配列へ整えます。
function normalizedToolDetails(event) {
    return (event.toolDetails || [])
        .map((tool) => tool.trim())
        .filter((tool, index, tools) => tool !== "" && tools.indexOf(tool) === index);
}
// statusLabel は保存値を画面表示用の日本語へ変換します。
function statusLabel(status) {
    const labels = {
        info: "情報",
        running: "実行中",
        success: "成功",
        failed: "失敗",
        blocked: "停止中",
    };
    return labels[status] || status;
}
// eventTypeLabel はイベント種別を日本語表示へ変換します。
function eventTypeLabel(type) {
    const labels = {
        task: "タスク",
        tool: "ツール",
        test: "テスト",
        question: "質問",
        error: "エラー",
    };
    return labels[type] || type;
}
// showHelp はカードの中で説明が切れないよう、共通の吹き出しを画面上に配置します。
function showHelp(target) {
    const tooltip = byId("tooltip-layer");
    const message = target.dataset.help;
    if (!message)
        return;
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
function hideHelp() {
    byId("tooltip-layer").hidden = true;
}
// 初心者向け説明はマウス操作とキーボード操作の両方で読めるようにします。
document.querySelectorAll("[data-help]").forEach((button) => {
    button.addEventListener("mouseenter", () => showHelp(button));
    button.addEventListener("focus", () => showHelp(button));
    button.addEventListener("mouseleave", hideHelp);
    button.addEventListener("blur", hideHelp);
});
window.addEventListener("scroll", hideHelp, { passive: true });
window.addEventListener("resize", hideHelp);
// フィルタ変更時は保存済みスナップショットを使い、再通信なしで表示だけ切り替えます。
document.querySelectorAll(".filter").forEach((button) => {
    button.addEventListener("click", () => {
        currentFilter = button.dataset.filter || "all";
        document.querySelectorAll(".filter").forEach((item) => item.classList.remove("active"));
        button.classList.add("active");
        if (currentSnapshot)
            renderEvents(currentSnapshot.events);
    });
});
// agent切替はAWS側のCodex/Claude別DynamoDBテーブルを切り替える操作です。
document.querySelectorAll(".agent-filter").forEach((button) => {
    button.addEventListener("click", () => {
        currentAgent = button.dataset.agent || "codex";
        document.querySelectorAll(".agent-filter").forEach((item) => item.classList.remove("active"));
        button.classList.add("active");
        void loadSnapshot();
    });
});
// 手動更新と自動更新の両方で同じ取得処理を使います。
byId("refresh").addEventListener("click", () => {
    void loadSnapshot();
});
byId("logout").addEventListener("click", logout);
byId("login-form").addEventListener("submit", (event) => {
    void login(event);
});
// startDashboard はログイン直後と初期表示で同じ監視ループを開始します。
function startDashboard() {
    if (refreshTimer)
        window.clearInterval(refreshTimer);
    void loadSnapshot();
    refreshTimer = window.setInterval(() => void loadSnapshot(), 5000);
}
if (ensureAuthenticated()) {
    startDashboard();
}
export {};
