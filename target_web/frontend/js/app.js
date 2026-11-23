"use strict";

// 沿用访问主机名，局域网访问时不会跳转到客户端的 localhost。
for (const [scheme, port] of [["http", "38080"], ["https", "38443"]]) {
  const url = new URL(window.location.href);
  url.protocol = `${scheme}:`;
  url.port = port;
  url.pathname = "/";
  url.search = "";
  url.hash = "";
  const link = document.getElementById(`${scheme}-link`);
  link.href = url.href;
  if (window.location.protocol === `${scheme}:`) link.setAttribute("aria-current", "page");
}
document.getElementById("current-url").textContent = window.location.origin;

const list = document.getElementById("api-list");
const reload = document.getElementById("reload-catalog");
const catalogStatus = document.getElementById("catalog-status");
const panel = document.getElementById("response-panel");
const badge = document.getElementById("response-badge");
const status = document.getElementById("request-status");
const details = document.getElementById("response-details");
let busy = false;
let focusBeforeBusy = null;

function setBusy(value) {
  if (value) {
    const active = document.activeElement;
    focusBeforeBusy = active === reload || list.contains(active) ? active : null;
  }
  busy = value;
  reload.disabled = value;
  for (const button of list.querySelectorAll("button")) button.disabled = value;
  // 禁用按钮会使浏览器失焦；只有用户未转移焦点时才恢复，避免抢焦点。
  if (!value && focusBeforeBusy?.isConnected && document.activeElement === document.body) {
    focusBeforeBusy.focus({ preventScroll: true });
  }
  if (!value) focusBeforeBusy = null;
}

async function fetchWithTimeout(path, options = {}) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 10000);
  try {
    const response = await fetch(path, { ...options, cache: "no-store", signal: controller.signal });
    // 正文读取也包含在超时范围内。
    const body = await response.text();
    return { response, body };
  } catch (error) {
    if (controller.signal.aborted) throw new Error("请求超过 10 秒，请检查服务状态后重试。");
    throw error;
  } finally {
    clearTimeout(timer);
  }
}

function formatBody(body) {
  if (!body) return "（响应正文为空）";
  try { return JSON.stringify(JSON.parse(body), null, 2); }
  catch { return body; }
}

function showResult(code, headers, body, elapsed) {
  document.getElementById("response-code").textContent = String(code);
  document.getElementById("response-time").textContent = `${Math.round(elapsed)} ms`;
  document.getElementById("response-headers").textContent = headers || "（无可读取的响应头）";
  document.getElementById("response-body").textContent = body;
  details.hidden = false;
}

async function invoke(entry, button) {
  if (busy) return;
  setBusy(true);
  for (const item of list.querySelectorAll("button")) item.setAttribute("aria-pressed", String(item === button));
  panel.setAttribute("aria-busy", "true");
  details.hidden = true;
  badge.textContent = "请求中";
  badge.dataset.tone = "neutral";
  status.dataset.tone = "neutral";
  status.textContent = "正在等待服务器响应…";
  document.getElementById("request-summary").textContent = `${entry.method} ${entry.path}`;
  const relay = entry.browser_mode === "relay";
  document.getElementById("request-route").textContent = relay
    ? `浏览器 POST /api/invoke → 后端 ${entry.method} ${entry.path}（内部 HTTP）`
    : `浏览器 ${entry.method} ${entry.path} · ${window.location.protocol.replace(":", "").toUpperCase()}`;
  if (window.matchMedia("(max-width: 800px)").matches) {
    panel.scrollIntoView({ behavior: "instant", block: "start" });
    panel.focus({ preventScroll: true });
  }

  const started = performance.now();
  try {
    const options = { method: relay ? "POST" : entry.method };
    if (relay || ["POST", "PUT", "PATCH"].includes(entry.method)) {
      options.headers = { "Content-Type": "application/json" };
      options.body = relay ? JSON.stringify({ method: entry.method }) : "{}";
    }
    const { response, body } = await fetchWithTimeout(relay ? "/api/invoke" : entry.path, options);
    let code = response.status;
    let responseBody = body;
    let headers = [...response.headers].map(([name, value]) => `${name}: ${value}`).join("\n");
    let relayed = false;
    if (relay && response.ok) {
      const result = JSON.parse(body);
      if (result.via !== "backend" || result.method !== entry.method || result.path !== entry.path || !Number.isInteger(result.status) || result.status < 100 || result.status > 599 || typeof result.body !== "string" || !result.headers || typeof result.headers !== "object") {
        throw new Error("代发接口返回的数据格式不正确，请重试。");
      }
      code = result.status;
      responseBody = result.body;
      headers = Object.entries(result.headers).map(([name, values]) => `${name}: ${Array.isArray(values) ? values.join(", ") : values}`).join("\n");
      relayed = true;
    }
    const expectedConnect = relayed && entry.method === "CONNECT" && code === 501;
    const success = code >= 200 && code < 300;
    badge.textContent = expectedConnect ? "501 · 预期响应" : `${code} · ${success ? "已响应" : "请求失败"}`;
    badge.dataset.tone = expectedConnect ? "warning" : success ? "success" : "error";
    status.dataset.tone = success || expectedConnect ? "neutral" : "error";
    if (relayed) {
      status.textContent = `代发入口 HTTP ${response.status}；下方为后端真实 ${entry.method} 请求的响应。耗时包含浏览器与代发往返。`;
    } else if (relay) {
      status.textContent = `代发入口返回 HTTP ${code}，未取得目标接口响应。下方显示入口返回内容；可再次点击重试。`;
    } else if (entry.method === "HEAD" && success) {
      status.textContent = "HEAD 请求完成。此方法不返回正文，说明位于 X-Request-Description 响应头。";
    } else {
      status.textContent = success ? "请求完成。下方为服务器返回的响应。" : `服务器返回 HTTP ${code}。可再次点击接口重试。`;
    }
    showResult(code, headers, entry.method === "HEAD" && !responseBody ? "（HEAD 按 HTTP 语义不返回响应正文，请查看上方响应头。）" : formatBody(responseBody), performance.now() - started);
  } catch (error) {
    badge.textContent = "请求失败";
    badge.dataset.tone = "error";
    status.dataset.tone = "error";
    status.textContent = `${error instanceof TypeError ? "无法连接服务，请检查网络和容器状态。" : error.message} 再次点击接口即可重试。`;
  } finally {
    panel.setAttribute("aria-busy", "false");
    setBusy(false);
  }
}

async function loadCatalog() {
  if (busy) return;
  setBusy(true);
  list.setAttribute("aria-busy", "true");
  list.replaceChildren();
  catalogStatus.hidden = false;
  catalogStatus.dataset.tone = "neutral";
  catalogStatus.textContent = "正在加载接口…";
  try {
    const { response, body } = await fetchWithTimeout("/api");
    if (!response.ok) throw new Error(`接口目录返回 HTTP ${response.status}。`);
    const catalog = JSON.parse(body);
    if (!Array.isArray(catalog.methods) || catalog.methods.length === 0) throw new Error("接口目录为空或格式不正确。");
    const fragment = document.createDocumentFragment();
    const seen = new Set();
    for (const entry of catalog.methods) {
      if (!/^(GET|HEAD|POST|PUT|PATCH|DELETE|OPTIONS|TRACE|CONNECT)$/.test(entry.method) || entry.path !== `/api/${entry.method.toLowerCase()}` || typeof entry.description !== "string" || entry.browser_mode !== (["TRACE", "CONNECT"].includes(entry.method) ? "relay" : "direct") || seen.has(entry.method)) {
        throw new Error("接口目录包含不支持的条目。");
      }
      seen.add(entry.method);
      const button = document.createElement("button");
      button.type = "button";
      button.className = "api-button";
      button.setAttribute("aria-pressed", "false");
      button.setAttribute("aria-controls", "response-panel");
      const line = document.createElement("span");
      line.className = "method-line";
      for (const [className, text] of [["method", entry.method], ["method-path", entry.path], ["method-mode", entry.browser_mode === "relay" ? "后端代发" : "浏览器直发"]]) {
        const item = document.createElement(className === "method-path" ? "code" : "span");
        item.className = className;
        item.textContent = text;
        line.append(item);
      }
      const description = document.createElement("span");
      description.className = "method-description";
      description.textContent = entry.description;
      button.append(line, description);
      button.addEventListener("click", () => invoke(entry, button));
      fragment.append(button);
    }
    list.append(fragment);
    catalogStatus.hidden = true;
  } catch (error) {
    catalogStatus.dataset.tone = "error";
    catalogStatus.textContent = `${error instanceof TypeError ? "无法加载接口目录，请检查服务连接。" : error.message} 点击“刷新列表”重试。`;
  } finally {
    list.setAttribute("aria-busy", "false");
    setBusy(false);
  }
}

reload.addEventListener("click", loadCatalog);
loadCatalog();
