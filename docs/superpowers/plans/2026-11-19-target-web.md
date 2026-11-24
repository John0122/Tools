# target_web Implementation Plan

> **For agentic workers:** 使用 `superpowers:executing-plans` 按任务顺序实施；本次在当前工作区交付可检查的文件，不提交或推送。

**Goal:** 实现 Docker Web 靶机及宿主机 IP HTTPS 证书。

**Architecture:** Nginx 提供 HTTP/HTTPS 页面并代理 Go API，8080 同时用于容器健康检查；宿主机 Bash 脚本检测地址、生成证书并调用 Compose。

**Tech Stack:** Nginx、Docker Compose v2、Bash、OpenSSL；Python 标准库仅用于测试。

**Spec:** `docs/superpowers/specs/2026-11-19-target-web-design.md`

## Global Constraints

- 前端映射 `38080:8080` 与 `38443:8443`，通过 HTTP/HTTPS 提供外部页面与接口。
- ECDSA P-256，自签名，825 天，SAN 包含访问 IPv4、localhost、127.0.0.1。
- 支持 macOS/Linux；证书与私钥不入库。

## Review Focus

- 默认路由无法检测：明确提示使用 `--host`。
- 非法 IPv4：失败且不覆盖已有证书。
- 重复启动：复用有效证书。
- 访问 IP 变化或密钥不匹配：重建证书，实际 TLS 握手验证。
- HTTP 和 HTTPS 两个端口对外可访问，验证证书信任和地址，不只检查跳过校验的响应。

## Task 1：证书与启动脚本

**Files:** `target_web/scripts/generate-cert.sh`、`target_web/target_web.sh`、`target_web/tests/test_cert.py`

**Interfaces:** `generate-cert.sh IPv4 证书目录 [--force]` 生成 `fullchain.pem` 和 `privkey.pem`；`target_web.sh up|down|status|logs|cert [--host IPv4]` 管理服务。

- [x] 编写证书行为测试，运行 `python3 -m unittest discover -s target_web/tests -v`，确认缺少实现时失败。
- [x] 实现证书生成、复用、失配重建及宿主机地址检测。
- [x] 运行同一测试命令，8 项全部通过；Bash 语法检查通过。

## Task 2：Docker 服务与说明

**Files:** `target_web/Dockerfile`、`target_web/compose.yaml`、`target_web/nginx/default.conf`、`target_web/html/index.html`、`target_web/README.md`、`target_web/.dockerignore`、`target_web/.gitignore`

**Interfaces:** `/` 为访问页，`/healthz` 返回服务与协议状态 JSON，证书挂载到 `/etc/nginx/certs`。

- [x] 实现 Nginx 配置、基础页面和两个端口映射；编写启动及证书验证说明。
- [x] 运行 `docker compose config --quiet`；执行 `./target_web.sh up` 并确认容器健康。
- [x] 使用宿主机 IP、127.0.0.1、localhost 请求两个协议的首页及健康检查，共 12 个请求均为 200；HTTPS 使用生成证书校验信任和地址；`nginx -t` 通过。
- [x] 审查最终变更并记录检查结果；独立复核未发现 Critical/Important 问题，文档命令问题已修正。

## 验证补充

- 从 `/tmp` 调用启动脚本，重复启动后容器健康，证书指纹不变。
- 证书、私钥、Python 缓存已验证被 Git 忽略；构建上下文仅包含配置和页面。
- 浏览器访问工具不可用，未完成可视化检查；页面内容已通过实际 HTTP 响应检查。
- 代码复核发现备用 Compose 命令应强制重建以载入手动续签后的证书，已同步文档。
- 已执行手动续签及直接 Compose 重建；两个端口的健康检查通过，实际 TLS 连接提供的证书与续签文件一致。
