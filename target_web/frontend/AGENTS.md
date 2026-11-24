# AGENTS

## 技术栈

- 前端使用原生 HTML、JavaScript、CSS；不引入 React、TypeScript、Vite 或组件库，不需要前端构建工具。
- 首页在 `index.html`，交互在 `js/app.js`；外链目录与分类页在 `links/`，共享交互在 `js/link-lab.js`。样式复用 `css/style.css` 的变量，分类页补充样式在 `css/link-lab.css`；设计约定见 `../DESIGN.md`。
- `../build.sh` 整理 `html/` 和 `nginx/` 临时产物上下文，再独立构建 Nginx 镜像；Dockerfile 只复制运行文件，不运行编译命令。Nginx 配置在 `nginx/`，不得复制进静态公开目录。
- Nginx 将 API 代理到 `backend:8000`，通过 Docker DNS 解析跟踪后端地址变化。
- 受控域名复用当前 Nginx 容器，`TARGET_WEB_EXTERNAL_HOST` 对应入口直接提供完整首页、分类页和同源 API，与 IP 入口保持路由一致；不存在的路径保留 404。外链清单以 IP 入口为对照，域名入口下指向同域名的链接属于站内。启动脚本替换静态产物与虚拟主机模板中的占位符；源码保留占位符，不硬编码部署 IP。
- IP 分类使用 `TARGET_WEB_HOST` 与可选 `TARGET_WEB_LAN_IP_ORIGIN`，公网链接固定为 `https://1.1.1.1/cdn-cgi/trace`。部署脚本提示输入另一台设备的私有 IPv4，或使用 `--lan-ip`，默认生成 HTTPS 38443 入口；入口校验器与启动脚本一并打包。局域网目标不能重复本站 IP；空配置不得生成可点击的空链接或伪造响应状态。
- 对外映射 `38080:8080` 与 `38443:8443`，同时提供 HTTP 与 HTTPS；容器内 HTTP 8080 也用于健康检查。本机 IP 证书由宿主机生成并只读挂载，证书与私钥不进入镜像。

## 中文

- 界面文字须有简体中文。
- 技术标识不得作为唯一说明；HTTP 方法、路径、响应头等技术内容配合中文用途或标题展示。

## 代码与交互

- 接口列表从后端 `/api` 加载，不硬编码模拟响应；使用同源相对路径发起真实请求。
- 外链演示使用真实 `/lab/` 请求及 `fixtures/` 资源；表单保留原生 action/formaction，WebSocket 使用真实 WS/WSS 连接。HTTP 页面提供 WS 38080 与 WSS 38443，HTTPS 页面只启用 WSS；域名资源默认使用 HTTPS。
- `../scripts/generate-link-pages.py` 是分类 HTML 与 `links/cases.json` 的生成来源，修改页面内容后同步生成产物；普通镜像构建不运行生成器。演示资源生成器与许可证见 `fixtures/SOURCES.md`。
- 暗链是有意预置的数据：只在指定样例中使用隐藏 CSS，并退出键盘焦点序列；正常控件保持可访问。死链保留真实 404/410，不增加首页回退。
- 分类页资源访问链接在新标签页打开，静态 HTML 与动态 DOM 同步设置 `target="_blank" rel="noopener"`；文件沿用浏览器下载行为。
- GET、HEAD、POST、PUT、PATCH、DELETE、OPTIONS 由浏览器直接发送；TRACE、CONNECT 经 `/api/invoke` 后端代发，并明确显示调用方式及目标状态。
- 请求期间禁用重复发送，展示等待、失败、超时和重试状态；状态不能只用颜色表达。
- 动态内容通过 `textContent` 等安全 DOM API 渲染，不将服务器响应作为 HTML 插入。
- 使用原生可访问控件、语义标题和可见键盘焦点；请求结束后合理保留焦点；窄屏跳到结果区域时同步移动焦点。
- 使用 `css/style.css` 已有主题变量；维护浅色、深色与窄屏布局，不新增平行样式系统。

## 验证与命令

- 当前没有 pnpm、ESLint、TypeScript 或前端打包脚本，不套用 FutureConsole 的框架检查命令。
- JavaScript 基础语法检查可在本目录使用 `node --check js/app.js` 与 `node --check js/link-lab.js`（需已安装 Node.js）；不得为纯语法检查擅自安装依赖。
- Nginx 配置修改后，在已有前端容器中可用 `docker compose -f ../compose.yaml exec -T frontend nginx -t` 检查语法；若容器未加载修改，则在获准重建后检查。
- 实际构建镜像、启动服务和浏览器验证的授权遵循根目录规范；单个前端服务构建命令在 `target_web/` 中为 `./build.sh --frontend-only`。
