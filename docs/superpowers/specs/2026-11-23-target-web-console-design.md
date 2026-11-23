# Target Web 双镜像与 API 调用列表

- 用户已确认：前后端两个独立镜像、两个容器；前端原生 HTML/JS/CSS；页面以可点击列表真实调用 API 并展示返回结果。
- 前端 Nginx 发布 HTTP 38080、HTTPS 38443，加载原有本地 IP 证书；页面、`/api` 和 `/healthz` 同源。后端 Go 仅监听容器内 8000，不发布宿主机端口、不包含前端文件或证书。
- 浏览器直接 fetch GET、HEAD、POST、PUT、PATCH、DELETE、OPTIONS。TRACE、CONNECT 通过 `POST /api/invoke` 由后端向自身固定接口发起真实 HTTP 请求；不得接受任意 URL、路径、请求头或其他方法。UI 明示代发及其内部 HTTP 链路。
- 接口目录来自 `GET /api`，没有硬编码成功结果。点击后显示加载、状态码、往返耗时、响应头和正文；HEAD 无正文，CONNECT 501 保持原样。请求期间避免重复提交，设置超时，网络/HTTP 错误均可见并可重试。
- 保留 Target Web 标题和蓝灰配色；桌面采用接口列表与结果双栏，窄屏顺序排列。按钮、状态、原始响应均采用原生语义及安全文本渲染。
- Compose 使用 `frontend`、`backend` 服务。启动脚本通过 `--remove-orphans` 迁移旧 `web` 容器，保留证书目录及其他项目容器。
- 验证包括真实网络的后端代发测试、Go 竞态检查、两镜像隔离、两个外部端口、浏览器逐项点击和失败恢复。

## 构建与规范约定

- 根目录与前后端 AGENTS.md 沿用 FutureConsole 的通用规范，技术栈按本项目调整。
- 后端内部端口为 8000，入口仍为 HTTP 38080、HTTPS 38443。
- build.sh 参考 FutureConsole：本机 Go 交叉编译，原生前端整理静态产物；两个独立临时产物上下文，Dockerfile 只组装运行镜像。
- 后端使用 scratch 与二进制健康检查；Compose 只运行已有镜像，启动脚本支持 --no-build。
