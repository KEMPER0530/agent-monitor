const byId = (id) => {
  const element = document.getElementById(id);
  if (!element) throw new Error(`missing #${id}`);
  return element;
};
async function loadSnapshot() {
  const response = await fetch("/api/snapshot", { cache: "no-store" });
  if (!response.ok) throw new Error(`snapshot failed: ${response.status}`);
  render(await response.json());
}
function render(snapshot) {
  byId("metric-events").textContent = String(snapshot.summary.totalEvents);
  byId("metric-running").textContent = String(snapshot.summary.runningTasks);
  byId("metric-failed").textContent = String(snapshot.summary.failedEvents);
  byId("metric-questions").textContent = String(snapshot.summary.openQuestions);
  byId("metric-cost").textContent = `$${snapshot.summary.totalCostUsd.toFixed(4)}`;
  byId("metric-tokens").textContent = snapshot.summary.totalTokens.toLocaleString();
  byId("metric-tools").textContent = snapshot.summary.totalToolCalls.toLocaleString();
  byId("last-message").textContent = snapshot.summary.lastEventMessage || "No events yet.";
  byId("last-updated").textContent = `Updated ${new Date(snapshot.generatedAt).toLocaleString()}`;
  const events = byId("events");
  events.innerHTML = "";
  const newestFirst = [...snapshot.events].reverse();
  if (newestFirst.length === 0) {
    events.append(emptyState());
    return;
  }
  for (const event of newestFirst) {
    events.append(eventRow(event));
  }
}
function emptyState() {
  const element = document.createElement("p");
  element.className = "empty";
  element.textContent = "Monitoring is ready. Enable it with AGENT_MONITOR_ENABLED=true or touch .agent-monitor.";
  return element;
}
function eventRow(event) {
  const article = document.createElement("article");
  article.className = `event ${event.status}`;
  const marker = document.createElement("span");
  marker.className = "marker";
  marker.textContent = event.type;
  const body = document.createElement("div");
  const title = document.createElement("h3");
  title.textContent = event.title;
  const meta = document.createElement("p");
  meta.textContent = [
    event.status,
    event.agent,
    new Date(event.createdAt).toLocaleString(),
  ].filter(Boolean).join(" / ");
  body.append(title, meta);
  if (event.message) {
    const message = document.createElement("p");
    message.className = "message";
    message.textContent = event.message;
    body.append(message);
  }
  article.append(marker, body);
  return article;
}
byId("refresh").addEventListener("click", () => {
  void loadSnapshot();
});
void loadSnapshot();
setInterval(() => void loadSnapshot(), 5000);

