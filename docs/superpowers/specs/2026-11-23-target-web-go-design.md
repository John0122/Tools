# target_web 前后端拆分

- 按用户要求将页面拆为 `frontend/index.html`、`frontend/css/style.css`、`frontend/js/app.js`，保留现有页面及协议入口。
- `backend/` 使用 Go 标准库，提供描述型 API，不引入数据库和业务写操作。`GET /api` 列出接口；`/api/request` 接受九种标准方法；`/api/get`、`/api/post` 等专用路径供逐项测试。
- 覆盖 GET、HEAD、POST、PUT、PATCH、DELETE、OPTIONS、TRACE、CONNECT。普通方法返回中文 JSON 描述；HEAD 返回描述响应头且无正文；OPTIONS 返回 Allow；TRACE 返回说明而不回显请求头；CONNECT 返回 501 和说明，不建立代理隧道。未知方法或路径分别返回 405/404。
- 前端 Nginx 与 Go 使用独立镜像和容器；Nginx 对外仅映射 `38443:8443`，8080 保留容器内部健康检查。Go 仅监听内部 8000；浏览器通过 HTTPS 直接发送七种方法，TRACE/CONNECT 由固定目标代发接口真实调用。
- 证书生成、复用及启动命令沿用已有脚本，证书只读挂载到前端 `/etc/nginx/certs`。Go 不加载 TLS 或托管前端文件；服务退出时关闭监听与升级连接。
- Go 在宿主机交叉编译，两个 Dockerfile 分别从产物上下文组装运行镜像；后端使用 scratch 与公共 CA。源码、测试、本机证书及私钥不进入后端镜像。
- 验证使用 Go 标准库测试和真实 HTTPS 请求，覆盖所有方法、HEAD 无正文、OPTIONS Allow、CONNECT 标准 authority 请求、错误路径、静态资源与证书验证。
