# Target Web 外链检测样例

用户已确认执行：一个二级目录页，按第一张图的 12 类链接建立 12 个三级测试页；第二张图作为每页 HTML 标签及技术特征的实现清单，补齐真实后端行为。

## 页面与样例

- `/links/` 为二级目录；三级路径为 `/links/{navigation,javascript,css,images,media,api,websocket,iframe,downloads,fonts,forms,redirects}/`。
- 复用原有蓝灰主题、系统字体、深色模式与键盘焦点。目录使用列表，测试页使用章节、样例编号、技术标签和实际演示区域。
- 动态链接、社交元数据、canonical 归入导航或 JavaScript 页；资源提示归入对应资源页；不另增分类页。
- 每类包含有效站内/站外对照，以及站内暗链、站外暗链、站内 404、站外 410。暗链真实存在于 DOM，通过透明、屏外、隐藏容器等方式隐藏；不把不可见的 script/link 标签自动称为暗链。
- 固定样例编号和机器可读清单记录预期结果；隐藏目标不在页面其他可见链接中重复出现，避免扫描器仅靠显链发现它。清单属于明确的测试对照数据。
- HTML 文件中的链接可静态提取；动态样例额外使用 JS 创建 DOM、修改 href/src。请求结果必须来自实际响应。
- 三级页的资源访问链接在新标签页打开，涵盖站内外普通链接、动态链接、暗链、死链和重定向入口；静态及动态链接均设置 target 与 noopener，文件按浏览器下载行为处理。

## 服务端

- Go 8000 负责 `/lab/rest`、`/lab/graphql`、`/lab/form`、`/lab/ws`、`/lab/status/{404,410}`、`/lab/redirect/{状态码}/{internal,external}/{ok,missing}`。
- REST 回显有限的演示参数；表单解析 GET、urlencoded 和 multipart POST，不保存数据；GraphQL 使用固定演示 Schema；WebSocket 提供真实文本回显、心跳、限时连接和正常关闭。
- JSON/表单输入限制 16 KiB，WebSocket 消息限制 4 KiB；固定演示目标，不接受任意重定向 URL。
- `/lab/` 的 CORS 只允许配置的主站与测试站来源；WebSocket 在协议升级前独立检查 Origin。
- Nginx 提供静态资源、下载响应头、媒体 Range 与 MIME；使用 WebSocket 专用代理配置。
- 错误样例返回真实 404/410，API、资源和页面不存在时不回退到 200 首页。

## 外站与构建

- 主站沿用本机 IP/localhost；外站默认 `pz-target-web.com`，通过现有 Nginx 独立虚拟站点提供样例，HTTP 38080 / HTTPS 38443，仍为两个镜像和两个容器。
- `TARGET_WEB_EXTERNAL_HOST` 可配置外站主机名；启动时替换静态产物中的占位主机名，原始前端源码不依赖某台机器 IP。
- 证书 SAN 增加外站主机名；生成与复用检查同步更新。DNS/hosts 需要让浏览器及扫描器解析到部署主机；不擅自修改系统 hosts。
- 主站 IPv4 由启动脚本传入后端的允许来源配置。测试站同源 HTTP/HTTPS 均可访问，页面默认引用 HTTPS 外站资源，WS 示例分别适配 HTTP/HTTPS。
- 页面和演示资源保存在前端镜像；Go 继续在宿主机编译并打包到 scratch。只复制明确运行目录。
- 协议库使用需获准的 `graphql-go/graphql` 和 `coder/websocket`；不引入前端框架、Node 构建链或数据库。

## 验证

覆盖真实接口输入与错误、GraphQL 查询、WebSocket 升级/回显/来源拒绝、CORS、固定跳转目标、所有分类页与资源、HTTP/HTTPS 证书、404/410、媒体 Range、下载响应及浏览器交互。所有临时验收脚本在仓库外。
