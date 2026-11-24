# Target Web 外链检测样例

一个二级目录页，包含 12 个按 HTML 技术特征组织的分类页及 1 个 IP 地址链接分类页，共 13 类；演示使用真实资源和后端行为。

## 页面与样例

- `/links/` 为二级目录；三级路径为 `/links/{navigation,javascript,css,images,media,api,websocket,iframe,downloads,fonts,forms,redirects,ip-address}/`。
- 复用原有蓝灰主题、系统字体、深色模式与键盘焦点。目录使用列表，测试页使用章节、样例编号、技术标签和实际演示区域。
- 所有 HTML 页面使用与首页一致的内嵌 SVG 标签页图标（Tabler `brand-feedly`，`#0d8e91`）；生成器的目录、分类与辅助页面模板统一输出 `link[rel=icon]` 的 `data:` URL，不新增网络资源请求。
- 动态链接、社交元数据、canonical 归入导航或 JavaScript 页；资源提示归入对应资源页；不另增分类页。
- 前 12 类包含有效站内/站外对照，以及站内暗链、站外暗链、站内 404、站外 410。IP 分类使用独立的内站暗链和 404/410，不混入域名外站。暗链真实存在于 DOM，通过透明、屏外、隐藏容器等方式隐藏；不把不可见的 script/link 标签自动称为暗链。
- 固定样例编号和机器可读清单记录预期结果；隐藏目标不在页面其他可见链接中重复出现，避免扫描器仅靠显链发现它。清单属于明确的测试对照数据。
- HTML 文件中的链接可静态提取；动态样例额外使用 JS 创建 DOM、修改 href/src。请求结果必须来自实际响应。
- 三级页的资源访问链接在新标签页打开，涵盖站内外普通链接、动态链接、暗链、死链和重定向入口；静态及动态链接均设置 target 与 noopener，文件按浏览器下载行为处理。

## IP 地址链接

- 页面 `/links/ip-address/` 按内站、公网外站、局域网其他设备分组。仅使用字面 IPv4；内站优先匹配本站 IP，即使本站使用私有地址。
- 本站地址使用 `TARGET_WEB_HOST`，提供 HTTP 38080 和 HTTPS 38443 的有效样例及暗链、404/410。公网链接固定为 `https://1.1.1.1/cdn-cgi/trace`，由生成器直接写入 HTML 与清单，保留完整路径。
- `up` 在未配置局域网入口且有交互终端时提示输入另一套服务的私有 IPv4，默认生成 `https://<IP>:38443/`；回车跳过，非法输入重试，输入流中断则取消部署。`--lan-ip` 可供自动部署使用，仅适用于 `up`，优先于已有 `TARGET_WEB_LAN_IP_ORIGIN` 配置；非交互且未配置时跳过，不阻塞部署。
- 局域网 IP 仅接受 RFC 1918 地址且不能重复本站 IP；裸 IP 输入不得带协议、端口或路径。原有 `TARGET_WEB_LAN_IP_ORIGIN` 继续接受 `http(s)://IPv4[:端口]` 和可选末尾 `/`，拒绝域名、认证信息、路径、查询和片段，端口为 1–65535。输入不写入持久文件，下一次部署重新输入或显式传入。
- 本站和局域网配置在前端启动时同时注入页面与清单，不依赖浏览器执行 JavaScript；IP 页不加载受控域名元数据及其演示脚本。局域网未配置时显示说明、清单 `target` 为 null；公网和局域网的 `expected_status` 为 null，不固定承诺 HTTP 200，也不自动访问目标。
- 清单共 144 条记录，IP 分类包含 7 条。`ip_group` 表示 `internal`、`public`、`lan`，保留 `scope` 的站点归属语义；记录内站 HTTP/HTTPS 和外站 WS/WSS 入口。
- 使用原生链接和既有章节、导航、主题；复用生成器与双容器结构，不引入额外依赖或代理接口。

## 服务端

- Go 8000 负责 `/lab/rest`、`/lab/graphql`、`/lab/form`、`/lab/ws`、`/lab/status/{404,410}`、`/lab/redirect/{状态码}/{internal,external}/{ok,missing}`。
- REST 回显有限的演示参数；表单解析 GET、urlencoded 和 multipart POST，不保存数据；GraphQL 使用固定演示 Schema；WebSocket 提供真实文本回显、心跳、限时连接和正常关闭。
- JSON/表单输入限制 16 KiB，WebSocket 消息限制 4 KiB；固定演示目标，不接受任意重定向 URL。
- `/lab/` 的 CORS 只允许配置的主站与测试站来源；WebSocket 在协议升级前独立检查 Origin。
- Nginx 提供静态资源、下载响应头、媒体 Range 与 MIME；使用 WebSocket 专用代理配置。
- 错误样例返回真实 404/410，API、资源和页面不存在时不回退到 200 首页。

## 外站与构建

- 主站沿用本机 IP/localhost；外站默认 `www.pz-target-web.com`，通过现有 Nginx 独立虚拟站点提供样例，对外映射 `38080:8080` 与 `38443:8443`，仍为两个镜像和两个容器。HTTP 8080 同时用于容器健康检查。
- IP 与域名入口的根路径均直接提供完整靶机首页，域名站点同步提供 `/links/`、静态资源、`/api`、`/lab/` 和 `/healthz`，与 IP 入口的路由一致；保留现有演示资源路径及真实 404/410。
- 清单的站内/站外标记以 IP 入口为对照；通过域名访问时，同域资源属于站内，页面和文档明确这一归属变化。
- `TARGET_WEB_EXTERNAL_HOST` 可配置外站主机名；启动时替换静态产物中的占位主机名，原始前端源码不依赖某台机器 IP。
- 证书 SAN 增加外站主机名；生成与复用检查同步更新。DNS/hosts 需要让浏览器及扫描器解析到部署主机；不擅自修改系统 hosts。
- 主站 IPv4 由启动脚本同步传入前端 IP 样例与后端的允许来源配置。测试站通过 HTTP/HTTPS 访问，跨域来源精确允许 HTTP 38080 与 HTTPS 38443；域名分类页引用 HTTPS 外站资源，HTTP 页面可测试 WS/WSS，HTTPS 页面仅启用 WSS。
- 页面和演示资源保存在前端镜像；Go 继续在宿主机编译并打包到 scratch。只复制明确运行目录。
- 协议库使用需获准的 `graphql-go/graphql` 和 `coder/websocket`；不引入前端框架、Node 构建链或数据库。

## 验证

覆盖真实接口输入与错误、GraphQL 查询、WS/WSS 升级/回显/来源拒绝、CORS、固定跳转目标、所有分类页与资源、HTTPS 证书、404/410、媒体 Range、下载响应及浏览器交互。所有临时验收脚本在仓库外。
