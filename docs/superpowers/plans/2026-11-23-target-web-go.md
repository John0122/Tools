# target_web Go Implementation Plan

> **For agentic workers:** 按 `superpowers:executing-plans` 在当前工作区实施；保留已有未提交文件，本次不提交或推送。

**Goal:** 拆分原生前端与 Go 后端，实现全部九种标准 HTTP 方法的描述接口。

**Architecture:** Go 同时服务静态资源及 API，HTTP/HTTPS 共用处理器；保留 Docker、端口和宿主机证书脚本。

**Tech Stack:** HTML、CSS、JavaScript、Go 标准库、Docker Compose、OpenSSL。

**Spec:** `docs/superpowers/specs/2026-11-23-target-web-go-design.md`

## Global Constraints

- 原生前端位于 `frontend/`，Go 后端位于 `backend/`。
- 宿主机 HTTP 38080、HTTPS 38443；沿用本机 IP 自签名证书。
- API 只返回描述，不执行数据写入或代理转发；HEAD 无正文，CONNECT 501 描述占位。

## Review Focus

- 所有方法可以通过两个公开端口到达接口。
- HEAD、OPTIONS、CONNECT 的响应符合文档约定。
- 错误方法和未知路径不会落入首页。
- CSS/JS 从新目录正确发布，源代码与证书不能作为静态文件访问。
- 任一监听或证书初始化失败时退出，避免仅启动一个协议却显示健康。

## Task 1：Go API 与服务

**Files:** `backend/go.mod`、`backend/internal/web/handler.go`、`backend/internal/web/handler_test.go`、`backend/cmd/server/main.go`

**Interfaces:** `web.NewHandler(frontend fs.FS) http.Handler`；启动参数指定前端目录、HTTP/HTTPS 地址及证书路径。

- [x] 先编写九种方法、专用路径、错误处理及真实 HTTP 连接的测试，确认缺少实现时失败。
- [x] 实现方法目录、描述响应、静态服务和双监听启动/关闭。
- [x] `go test -race ./...`、`go vet ./...`、`go build ./...` 均通过。

## Task 2：目录与 Docker

**Files:** `frontend/`、`backend/Dockerfile`、`compose.yaml`、`.dockerignore`、`README.md`。

- [x] 抽离页面脚本和样式，添加接口目录入口，删除旧 html/nginx 目录及根 Dockerfile。
- [x] 更新 Docker 构建和证书挂载，更新目录与 API 使用说明。
- [x] 构建并启动，验证两个端口的所有方法及前端文件，HTTPS 校验真实证书。
- [x] 完成静态检查和独立复核，记录可用与不可用的验证结果。

## 验证记录

- Go 测试（含竞态检测）、`go vet`、编译及 Docker 多阶段构建通过；证书回归 8 项通过。
- 真实端口共 64 项检查通过：两种协议的九种方法及专用路径、接口目录、静态 HTML/JS/CSS、OPTIONS 星号目标、CONNECT authority 目标、404/405 和三个访问地址的健康检查。HTTPS 全程使用生成证书校验信任及地址。
- 独立复核无 Critical/Important；发现依次 Shutdown 会让 HTTPS 延迟停止接收请求，已改为并行关闭。新增在途 HTTP 请求的回归测试，旧实现失败，新实现通过。
- 前端 strict 静态审查无发现。浏览器连接工具不可用，改用本机 Chrome 实际检查页面样式、JS 生成的当前地址和接口目录跳转，均正常。
- 运行中的容器收到 Docker SIGTERM 后以 0 退出；重新启动达到 healthy，HTTP/HTTPS 健康检查再次通过。
