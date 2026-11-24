# Target Web Console Implementation Plan

> **For agentic workers:** 在当前工作区按 `superpowers:executing-plans` 实施，不提交或推送。

**Goal:** 交付双镜像部署和真实 API 调用列表。

**Architecture:** Nginx 托管前端并代理内部 Go API；受浏览器限制的方法由 Go 向自身固定接口代发。

**Tech Stack:** 原生 HTML/JS/CSS、Nginx、Go 标准库、Docker Compose。

**Spec:** `docs/superpowers/specs/2026-11-23-target-web-console-design.md`

## Global Constraints

- 前端发布 `38080:8080` 与 `38443:8443` 并加载本机证书，8080 同时用于容器健康检查，后端端口不发布。
- 九种方法均可点击；TRACE/CONNECT 明确标注代发，CONNECT 的目标响应为 501。
- 返回结果来自真实请求；后端代发目标不接受用户输入。

## Review Focus

- 两个镜像的构建上下文和运行文件真正分离。
- 旧 web 容器迁移不会占用前端端口。
- 错误方法、任意目标、超大/错误 JSON 不进入代发流程。
- HEAD、501、网络错误、重复点击与超时的 UI 状态清楚。
- 前端通过相同源代理调用 API，后端重建后代理可恢复。

## Task 1：后端与容器拆分

- [x] 更新测试，覆盖 API-only、真实 TRACE/CONNECT 代发、输入拒绝及服务停止；先观察预期失败。
- [x] 去掉 Go 静态服务与 TLS；固定自身代发目标，保留原有方法描述接口。
- [x] 新建前端 Dockerfile/Nginx，缩小后端构建上下文，更新 Compose 和启动脚本。
- [x] Go 测试、vet、构建与 Docker 配置检查通过。

## Task 2：可点击列表与交付验证

- [x] 实现目录加载、真实请求、结果展示、加载/失败/重试和响应式布局。
- [x] 更新设计约定及 README。
- [x] 构建迁移并检查双镜像、双端口、证书及后端隔离。
- [x] 实际浏览器验证九种方法、桌面键盘和 393px 窄屏布局；真实 HTTP 验证后端停机、地址变化后的代理恢复；完成代码复核。

## 构建与规范约定

- 根目录与前后端 AGENTS.md 沿用 FutureConsole 的通用规范，技术栈按本项目调整。
- 后端内部端口为 8000，外部入口为 HTTP 38080 和 HTTPS 38443。
- build.sh 参考 FutureConsole：本机 Go 交叉编译，原生前端整理静态产物；两个独立临时产物上下文，Dockerfile 只组装运行镜像。
- 后端使用 scratch 与二进制健康检查；Compose 只运行已有镜像，启动脚本支持 --no-build。

- [x] 统一后端端口为 8000，补充二进制健康检查及其测试。
- [x] 按 FutureConsole 的产物构建流程重建两个镜像并启动，通过 HTTP/HTTPS 与镜像内容检查。
- [x] 拷贝并适配根目录、前端与后端规范，更新构建和运行文档。
