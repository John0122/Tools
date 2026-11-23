"use strict";

// 分类页共用交互；所有结果取自实际请求，外站主机名在容器启动时注入。
(() => {
  const external = new URL(document.querySelector('meta[name="lab-external-base"]').content);
  const networkHelp = "未能取得响应，请检查外站域名解析、HTTPS 证书信任和服务状态。";
  const format = (body) => { try { return JSON.stringify(JSON.parse(body), null, 2); } catch { return body || "（空响应）"; } };

  async function request(url, method, body, kind) {
    if (kind === "xhr") {
      return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open(method, url); xhr.timeout = 8000;
        if (body) xhr.setRequestHeader("Content-Type", "application/json");
        xhr.onload = () => resolve({ code: xhr.status, headers: xhr.getAllResponseHeaders(), body: xhr.responseText });
        xhr.onerror = () => reject(new Error(networkHelp));
        xhr.ontimeout = () => reject(new Error("请求超过 8 秒，可检查服务后重试。"));
        xhr.send(body || null);
      });
    }
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 8000);
    try {
      const response = await fetch(url, { method, body: body || undefined, headers: body ? { "Content-Type": "application/json" } : undefined, signal: controller.signal, cache: "no-store" });
      const text = await response.text();
      return { code: response.status, headers: [...response.headers].map(([k, v]) => `${k}: ${v}`).join("\n"), body: text };
    } catch (error) {
      throw new Error(controller.signal.aborted ? "请求超过 8 秒，可检查服务后重试。" : networkHelp);
    } finally { clearTimeout(timer); }
  }

  for (const button of document.querySelectorAll("[data-request]")) {
    button.disabled = false;
    const result = document.getElementById(button.getAttribute("aria-controls"));
    const run = async () => {
      if (button.disabled) return;
      const focused = document.activeElement === button;
      button.disabled = true; result.setAttribute("aria-busy", "true"); result.dataset.tone = "neutral";
      result.textContent = "正在请求外站…";
      const started = performance.now();
      try {
        const response = await request(button.dataset.request, button.dataset.method, button.dataset.body, button.dataset.requestKind);
        result.dataset.tone = response.code < 400 ? "success" : "error";
        result.textContent = `HTTP ${response.code} · ${Math.round(performance.now() - started)} ms\n${response.headers}\n\n${format(response.body)}`;
      } catch (error) { result.dataset.tone = "error"; result.textContent = `${error.message} 可再次点击“发送请求”。`; }
      finally { button.disabled = false; result.setAttribute("aria-busy", "false"); if (focused && document.activeElement === document.body) button.focus({ preventScroll: true }); }
    };
    button.addEventListener("click", run);
    if (button.dataset.auto === "true") run();
  }

  for (const button of document.querySelectorAll("[data-create-link]")) {
    button.disabled = false;
    button.addEventListener("click", () => {
      const anchor = document.createElement("a"); anchor.href = button.dataset.createLink;
      // 动态资源链接遵循静态样例的新标签页规则，目标页不持有当前页引用。
      anchor.target = "_blank"; anchor.rel = "noopener";
      anchor.textContent = "访问动态创建的外站资料"; anchor.dataset.caseId = button.dataset.caseId;
      button.parentElement.querySelector("[data-dynamic-result]").replaceChildren(anchor);
    });
  }
  for (const button of document.querySelectorAll("[data-load-script]")) {
    button.disabled = false;
    button.addEventListener("click", () => {
      if (button.disabled) return;
      const state = button.parentElement.querySelector("[data-script-result]");
      const script = document.createElement("script"); script.src = button.dataset.loadScript;
      button.disabled = true; state.textContent = "正在加载动态脚本…";
      let completed = false;
      const done = (message) => { if (completed) return; completed = true; clearTimeout(timer); button.disabled = false; state.textContent = message; script.remove(); };
      const timer = setTimeout(() => done("脚本加载超过 8 秒，请检查外站后重试。"), 8000);
      script.onload = () => done("动态脚本已通过真实 src 地址加载并执行。");
      script.onerror = () => done(networkHelp);
      document.head.append(script);
    });
  }

  // 保留浏览器原生 form/action/formaction 提交，避免把表单样例替换成 Fetch。
  for (const form of document.querySelectorAll("[data-lab-form]")) {
    const state = form.querySelector("[data-form-status]");
    const frame = form.parentElement.querySelector("[data-form-frame]");
    let pending = false, timer = null;
    const finish = (message) => { pending = false; clearTimeout(timer); for (const b of form.querySelectorAll("button")) b.disabled = false; state.textContent = message; form.setAttribute("aria-busy", "false"); };
    form.addEventListener("submit", (event) => {
      if (pending) { event.preventDefault(); return; }
      const name = form.elements.namedItem("name");
      name.setAttribute("aria-invalid", String(!name.value.trim()));
      if (!name.value.trim()) { event.preventDefault(); state.dataset.tone = "error"; state.textContent = "请填写演示姓名。"; name.focus(); return; }
      pending = true; form.setAttribute("aria-busy", "true"); state.dataset.tone = "neutral"; state.textContent = "正在通过浏览器原生表单提交…";
      // 序列化提交按钮后再禁用，保留 formaction 的选择结果。
      queueMicrotask(() => { if (pending) for (const b of form.querySelectorAll("button")) b.disabled = true; });
      timer = setTimeout(() => finish("8 秒内未收到页面加载通知，可检查目标地址后重试。"), 8000);
    });
    frame.addEventListener("load", () => { if (pending) finish("目标文档已加载，下方显示浏览器收到的响应。跨站响应状态可在网络面板检查。"); });
  }

  const socketPanel = document.querySelector("[data-websocket]");
  if (socketPanel) {
    let socket = null, connectTimer = null;
    const log = socketPanel.querySelector("[data-ws-log]");
    const send = socketPanel.querySelector("[data-ws-send]");
    const close = socketPanel.querySelector("[data-ws-close]");
    const buttons = [...socketPanel.querySelectorAll("[data-ws-connect]")];
    const lines = [];
    const note = (value) => { lines.push(value); log.textContent = lines.slice(-15).join("\n"); };
    const refresh = () => {
      const active = socket && socket.readyState < WebSocket.CLOSING;
      for (const b of buttons) b.disabled = Boolean(active) || (b.dataset.wsConnect === "ws" && location.protocol === "https:");
      send.disabled = !socket || socket.readyState !== WebSocket.OPEN;
      close.disabled = !active;
    };
    refresh();
    if (location.protocol === "https:") note("当前为 HTTPS 页面，请使用 WSS；WS 可从 HTTP 入口测试。");
    for (const b of buttons) b.addEventListener("click", () => {
      if (socket && socket.readyState < WebSocket.CLOSING) return;
      const protocol = b.dataset.wsConnect;
      const url = `${protocol}://${external.hostname}:${protocol === "wss" ? 38443 : 38080}/lab/ws`;
      const current = new WebSocket(url); socket = current; refresh(); note(`正在连接 ${url}`);
      // 每个连接持有自己的计时器，旧连接延迟关闭不能取消新连接的超时。
      const currentTimer = setTimeout(() => { if (current.readyState === WebSocket.CONNECTING) { note("连接超过 8 秒，请检查域名与证书。"); current.close(); } }, 8000);
      connectTimer = currentTimer;
      current.addEventListener("open", () => { clearTimeout(currentTimer); refresh(); note("连接已建立，可以发送消息。"); });
      current.addEventListener("message", (event) => note(`收到：${event.data}`));
      current.addEventListener("error", () => note(networkHelp));
      current.addEventListener("close", (event) => { clearTimeout(currentTimer); if (socket === current) socket = null; refresh(); note(`连接已关闭，代码 ${event.code}${event.reason ? "：" + event.reason : ""}`); });
    });
    send.addEventListener("click", () => { if (socket?.readyState === WebSocket.OPEN) { const text = document.getElementById("ws-message").value; socket.send(text); note(`发送：${text}`); } });
    close.addEventListener("click", () => { socket?.close(1000, "用户结束演示"); refresh(); });
    window.addEventListener("pagehide", () => { clearTimeout(connectTimer); socket?.close(1000, "离开演示页面"); });
  }
})();
