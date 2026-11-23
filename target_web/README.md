# target_web

通过 Docker 运行的 Web 靶机。前端使用原生 HTML、JavaScript、CSS，由 Nginx 提供页面和 HTTPS；Go 后端提供九种 HTTP 方法及外链检测所需的真实接口。前后端分别构建镜像、分别运行容器。

打开页面即可点击接口列表，真实发送请求，查看状态码、耗时、响应头和正文。

## 部署结构

```text
浏览器 ── HTTP :38080 / HTTPS :38443 ── frontend（Nginx）
                                          ├── 首页、/links/、/fixtures/
                                          └── /api、/lab/、/healthz ── backend（Go :8000）
```

| 服务 | 镜像 | 产物准备 | 监听与职责 |
| --- | --- | --- | --- |
| frontend | `target-web-frontend:local` | 整理 `frontend/` 静态文件 | 发布 38080→8080、38443→8443；静态文件、TLS、API 反向代理 |
| backend | `target-web-backend:local` | 本机交叉编译 `backend/` | 仅在 Docker 网络内监听 8000；API 与受限代发接口 |

HTTP 和 HTTPS 均直接提供页面，不强制重定向。浏览器使用同源 `/api`，无需配置跨域。后端不包含源码、前端文件或本机 IP 证书/私钥，不发布宿主机端口。Nginx 通过 Docker DNS 自动跟踪后端容器地址变化。

```text
target_web/
├── frontend/
│   ├── index.html
│   ├── css/style.css
│   ├── js/app.js
│   ├── links/                 # 二级目录、12 个三级分类页、样例清单
│   ├── fixtures/              # 真实演示资源与素材许可证
│   ├── docker-entrypoint.d/   # 注入受控外站主机名
│   ├── nginx/                 # 静态服务、HTTPS、反向代理配置
│   └── Dockerfile
├── backend/
│   ├── cmd/server/            # API 服务启动、关闭及测试
│   ├── internal/web/          # 方法描述、固定目标代发及测试
│   ├── internal/linklab/      # 外链样例接口、GraphQL、WebSocket
│   ├── go.mod
│   └── Dockerfile             # scratch 运行镜像，只复制二进制及公共 CA
├── scripts/generate-cert.sh
├── tests/test_cert.py
├── certs/                     # 本地生成，不入库、不进入构建上下文
├── DESIGN.md                  # 页面设计约定
├── build.sh                   # 本机编译、产物整理与双镜像构建
├── build/                     # 临时构建上下文，自动清理且不入库
├── compose.yaml
└── target_web.sh
```

## 启动

宿主机需要 Docker、Docker Compose v2（支持 `up --wait`）和 OpenSSL 1.1.1 或更新版本。启动脚本支持 macOS、Linux；Linux 自动检测需要 `ip` 命令。完整构建需要本机 Go 1.26.x（与 `backend/go.mod` 一致到次版本），无需 Node.js 或 pnpm。运行已有镜像时无需 Go。构建前检查目标平台的 `nginx:1.31-alpine-slim`、`alpine:3.24`，本地缺失时才拉取。

```bash
cd target_web
./target_web.sh up

# 多网卡、VPN 或指定证书 IP 时
./target_web.sh up --host 192.168.5.10
```

脚本检测宿主机默认路由 IPv4，生成或复用证书，通过 `build.sh` 在本机编译并构建两个镜像，等待两个容器健康后输出访问地址。`--host` 指定证书中的访问 IP，端口发布到宿主机所有接口。地址检测与手动输入目前使用 IPv4。

从旧版单容器升级时，脚本的 `--remove-orphans` 会移除同一 Compose 项目中的旧 `web` 容器，释放端口。证书保留并复用。

直接使用 Compose 时，先准备镜像与证书：

```bash
export TARGET_WEB_HOST=192.168.5.10
export TARGET_WEB_EXTERNAL_HOST=pz-target-web.com
./build.sh
./target_web.sh cert --host "$TARGET_WEB_HOST"
docker compose up -d --no-build --force-recreate --remove-orphans --wait
```

直接使用 Compose 时，上述环境变量须在同一终端保留，以便后端允许实际主站来源。通常使用 `target_web.sh up --host ...` 自动传入即可。

## 镜像编译与构建

构建流程参考 FutureConsole 的 `deploy/build.sh`、`deploy/backend.Dockerfile`、`deploy/web.Dockerfile`：

1. 检查 Docker 与 Go 版本，确定 Linux 目标架构，支持 `amd64`、`arm64`，默认与 Docker 服务一致。
2. 从两个 Dockerfile 读取、去重基础镜像，跳过 `scratch` 和命名阶段。按目标平台检查本地镜像，只对缺失的平台拉取；拉取失败或平台不符立即停止。
3. 在宿主机以 `CGO_ENABLED=0 GOOS=linux GOARCH=<架构> go build -trimpath -ldflags="-s -w"` 编译 Go。原生前端无需转译，按明确路径整理 HTML、JS、CSS 和 Nginx 配置。
4. 每个服务使用独立临时产物上下文 `build/context.<随机值>/<服务>/`。Docker 不接收 Go 源码、测试、规范或本机证书。
5. Dockerfile 只复制运行产物，不执行 `RUN`；后端运行于 `scratch`，从 Alpine 复制公共 CA，使用非 root 用户；前端运行于 Nginx。两个镜像都带 OCI 版本与提交号标签。
6. 使用当前 Docker context 的默认 builder，忽略 `BUILDX_BUILDER`，显式 `--pull=false`，复用已准备的基础镜像。构建结束或失败均清理临时上下文。

后端镜像没有 shell、Go 或 wget；健康检查由 `/server healthcheck` 请求本机 8000 端口完成。前端沿用 Nginx 主进程读取权限为 600 的 TLS 私钥，工作进程由官方配置以 nginx 用户运行。

```bash
./build.sh                       # 构建两个镜像，默认标签 local
./build.sh --frontend-only       # 只构建前端，无需 Go
./build.sh --backend-only        # 只编译、构建后端
./build.sh --arch amd64          # 指定 Linux 目标架构
./build.sh --base-images         # 查看基础镜像标签，不构建
./build.sh --version dev-demo    # 指定两个镜像的标签

# 只运行已构建或 docker load 导入的镜像，不在运行机编译
TARGET_WEB_IMAGE_TAG=dev-demo ./target_web.sh up --no-build --host 192.168.5.10
```

Compose 只负责运行已构建镜像，设置 `pull_policy: never`，不从仓库拉取业务镜像。默认标签由 `TARGET_WEB_IMAGE_TAG` 指定，未设置时为 `local`；`--version` 只影响本次构建，使用其他标签启动时需设置同一个环境变量。不要直接将源码目录传给这两个 Dockerfile；请使用 `build.sh` 准备上下文。

## 页面与 API

页面从 `GET /api` 加载目录；点击列表发送请求，并展示返回结果。请求等待期间禁用重复点击，超过 10 秒显示超时错误；请求失败可再次点击重试，目录加载失败可点“刷新列表”。POST、PUT、PATCH 的页面示例请求体为 `{}`。

| 方法 | 描述接口 | 页面请求方式 | 目标响应 |
| --- | --- | --- | --- |
| GET | `/api/get` | 浏览器直接 GET | 200，获取资源说明 |
| HEAD | `/api/head` | 浏览器直接 HEAD | 200，无正文，说明位于 `X-Request-Description` |
| POST | `/api/post` | 浏览器直接 POST | 200，提交数据说明 |
| PUT | `/api/put` | 浏览器直接 PUT | 200，替换资源说明 |
| PATCH | `/api/patch` | 浏览器直接 PATCH | 200，更新资源说明 |
| DELETE | `/api/delete` | 浏览器直接 DELETE | 200，删除资源说明 |
| OPTIONS | `/api/options` | 浏览器直接 OPTIONS | 200，方法说明及 `Allow` |
| TRACE | `/api/trace` | 浏览器 POST `/api/invoke`，Go 真实代发 TRACE | 200，链路诊断说明 |
| CONNECT | `/api/connect` | 浏览器 POST `/api/invoke`，Go 真实代发 CONNECT | 501，隧道方法说明 |

[Fetch 标准](https://fetch.spec.whatwg.org/) 禁止浏览器直接发出 TRACE、CONNECT。页面明确标注“后端代发”：Go 向自身监听地址的固定描述接口发起真实 HTTP 请求，再返回实际状态、响应头和正文。此内部请求使用 HTTP；页面通过 HTTPS 访问时，浏览器到 Nginx 的连接仍使用 HTTPS。Nginx 入口也不接受原始 TRACE/CONNECT，外部测试这两种方法请使用 `/api/invoke`。

代发接口只接受 `{"method":"TRACE"}` 或 `{"method":"CONNECT"}`，不接受 URL、路径、认证信息或其他字段，不转发浏览器 Cookie/Authorization，不跟随重定向。请求体最多 1024 字节，内部请求超时 5 秒。成功取得目标响应时入口返回 HTTP 200，JSON 的 `status` 才是目标状态；页面将两者分别说明，CONNECT 的预期 501 不会被显示成 200。

普通描述响应：

```json
{
  "service": "target_web",
  "method": "POST",
  "path": "/api/post",
  "description": "POST 请求用于提交数据或创建资源；当前接口仅返回说明，不保存数据。"
}
```

代发响应结构：

```json
{
  "via": "backend",
  "method": "CONNECT",
  "path": "/api/connect",
  "status": 501,
  "headers": {"Content-Type": ["application/json; charset=utf-8"]},
  "body": "目标接口返回的原始正文字符串"
}
```

九种方法用于说明演示，不保存请求体、不修改或删除数据；TRACE 不回显请求头，CONNECT 不建立代理隧道。Go 后端也保留 `/api/request` 统一入口和标准 CONNECT 的 `host:port` 请求目标形式。Nginx 入口可直接访问其中七种浏览器允许的方法。

`GET /api` 或 `/api/` 返回完整目录及 `browser_mode`（`direct` / `relay`）。HEAD 的说明响应头使用 ASCII 英文，中文说明在目录中。OPTIONS 的 `Allow` 与当前路径一致：查询全部后端方法用 `/api/request`；专用路径返回该路径支持的方法。`/api/get` 也接受 HEAD，各描述路径均接受 OPTIONS。

未知 API 路径返回 404，不支持的方法返回 405 及 `Allow`。方法名区分大小写；当前集合为上述九种，不含 WebDAV 扩展方法。

```bash
curl --noproxy '*' http://127.0.0.1:38080/api
curl --noproxy '*' http://127.0.0.1:38080/api/get
curl --noproxy '*' -I http://127.0.0.1:38080/api/head
curl --noproxy '*' -X POST -H 'Content-Type: application/json' -d '{}' http://127.0.0.1:38080/api/post
curl --noproxy '*' -X PUT http://127.0.0.1:38080/api/put
curl --noproxy '*' -X PATCH http://127.0.0.1:38080/api/patch
curl --noproxy '*' -X DELETE http://127.0.0.1:38080/api/delete
curl --noproxy '*' -i -X OPTIONS http://127.0.0.1:38080/api/request
curl --noproxy '*' -H 'Content-Type: application/json' -d '{"method":"TRACE"}' http://127.0.0.1:38080/api/invoke
curl --noproxy '*' -H 'Content-Type: application/json' -d '{"method":"CONNECT"}' http://127.0.0.1:38080/api/invoke
curl --noproxy '*' --cacert certs/fullchain.pem -X PATCH https://127.0.0.1:38443/api/request
```

## 外链检测页面

首页入口 → `/links/` 二级目录 → 12 个三级分类页。第一张分类表决定目录，HTML 标签与技术特征决定各页中的演示区域；不递归生成大量子页面。

三级页面的资源访问链接统一在新标签页打开，包括站内外普通链接、动态生成链接、暗链、死链、重定向入口及样例清单等资料链接，保留当前测试页面。文件链接由浏览器按下载响应处理。

| 三级路径（`/links/` 下） | 分类 | 实际样例 |
| --- | --- | --- |
| `navigation/` | 页面跳转 | a href、动态 DOM、canonical、Open Graph、Twitter Card |
| `javascript/` | JavaScript | script src、模块及依赖、动态加载、preload/prefetch/preconnect/dns-prefetch |
| `css/` | CSS | link stylesheet、@import、背景 url() |
| `images/` | 图片 | PNG/JPG/GIF/SVG/WebP、picture/source、srcset |
| `media/` | 音视频 | MP4、MP3、M3U8、source、外站播放器 |
| `api/` | API | Fetch GET、XHR POST、GraphQL 查询与变量、失效接口 |
| `websocket/` | WebSocket | WS/WSS 握手、文本回显、心跳、关闭 |
| `iframe/` | 嵌入页面 | 独立文档、地图示意、状态组件、失效 iframe |
| `downloads/` | 文件下载 | 有效 PDF/DOCX/ZIP/EXE 与附件响应头 |
| `fonts/` | 字体 | @font-face 加载 WOFF/WOFF2/TTF/OTF |
| `forms/` | 表单 | 原生 GET、POST urlencoded/multipart、formaction 覆盖 |
| `redirects/` | 重定向 | 301/302/303/307/308、Meta Refresh、JS 跳转、失效跳转链 |

每个分类页还包含站内暗链、站外暗链、站内 404、站外 410，以及正常折叠内容对照。暗链真实存在于 DOM，通过隐藏容器、透明或屏外定位隐藏；隐藏目标不在普通可见链接中重复出现。暗链表示可见性，死链表示可达性，站内/站外表示归属，这些维度分别记录。正常 script/link 资源声明不因为不可见就被当成暗链。

`/links/cases.json` 提供 137 条样例记录，包含固定编号、页面、技术特征、目标、方法、触发方式、可见性和预期 HTTP 状态，是结果核对清单；需要独立衡量发现能力时，不把清单作为扫描起点。跳转记录的状态是第一跳，仍应跟踪最终目标。DNS 预取与预连接没有 HTTP 响应，其方法和状态字段为 null。资源提示和社交元数据不保证浏览器主动请求；动态按钮、表单和 WS 需要实际交互触发。

图片、字体、媒体和下载均为实际文件；`sample.exe` 是仅打印说明的 Windows 程序。M3U8 引用有效 MP3 Packed Audio 分片，播放需要原生支持 HLS 的浏览器。预置 404/410 会出现在浏览器网络面板和控制台中，这是正常测试数据。

### 受控外站配置

外站默认 `pz-target-web.com`，与主站 IP 属于不同主机名/来源，但复用同一个前端容器、IP 和端口；无需第三个镜像。外站只提供演示资源和 `/lab/`，不提供主站首页或 `/api` 控制台。每个浏览器或扫描器所在设备都需配置 DNS/hosts，例如部署 IP 为 `192.168.5.10` 时：

```text
192.168.5.10 pz-target-web.com
```

部署脚本不修改系统 hosts 或证书信任。自签名证书同时覆盖主站 IP 和外站域名，浏览器需要允许或信任两个入口使用的证书，外站 HTTPS 子资源才能加载。可先访问 `https://pz-target-web.com:38443/fixtures/landing.html` 检查。页面默认引用 HTTPS 外站资源；HTTP 页面可测试 WS/WSS，HTTPS 页面只启用 WSS。

使用系统或浏览器代理时，将部署 IP 和外站域名加入直连列表；hosts 映射不能保证代理服务器也能解析该域名。命令行示例使用 `--noproxy '*'` 直接访问本地靶机。

可选自定义外站主机名（合法的小写 DNS 名称，不填协议或端口）：

```bash
TARGET_WEB_EXTERNAL_HOST=resources.lab.test ./target_web.sh up --host 192.168.5.10
```

域名变化会更新证书、前端运行产物与后端允许来源；也需同步各访问设备的解析设置。`TARGET_WEB_HOST` 由 `up --host` 或 IP 自动检测传入后端。无需改 hosts 的命令行检查：

```bash
curl --noproxy '*' --cacert certs/fullchain.pem \
  --resolve pz-target-web.com:38443:192.168.5.10 \
  https://pz-target-web.com:38443/fixtures/landing.html
```

### 后端样例契约

| 接口 | 方法 | 行为 |
| --- | --- | --- |
| `/lab/rest` | GET/HEAD/POST | 查询或 JSON 中的 name/message 原样返回，不保存 |
| `/lab/graphql` | POST JSON | Query service、article(id: ID!)、echo(message: String!)；有效 article ID 为 intro，未知 ID 为 null；无效查询返回 400 |
| `/lab/form` | GET/HEAD/POST | GET、urlencoded 或 multipart 文本字段，name 必填；返回实际收到的内容 |
| `/lab/ws` | GET 升级 | 真实 WebSocket 文本回显，4 KiB 消息限制、20 秒心跳、连接最多 2 分钟 |
| `/lab/status/404`、`/lab/status/410` | GET/HEAD | 对应真实错误状态，附 X-Lab-Case |
| `/lab/redirect/{code}/{scope}/{target}` | GET/HEAD | code 为 301/302/303/307/308，scope 为 internal/external，target 为 ok/missing；只跳转到固定演示目标 |

以上已知路径提供 OPTIONS；不支持的方法返回 405，未知路径返回 404。JSON 与表单请求体上限 16 KiB；只接收演示文本，不接收文件上传。`/lab/` 允许的来源为配置主站 IP、localhost/回环和外站主机名对应的 38080/38443 入口，未配置来源与 `Origin: null` 返回 403。公开静态演示资源允许无凭据跨域；媒体支持 Range，文件返回正确类型及 Content-Disposition。重定向不接受任意 URL。

协议依赖锁定为 `github.com/graphql-go/graphql v0.8.1`、`github.com/coder/websocket v1.8.15`，没有前端第三方运行依赖。媒体、照片与字体的来源、独立许可证见 [资源说明](frontend/fixtures/SOURCES.md)。

### 更新演示内容

生成的静态文件已保存在仓库，正常构建镜像无需 Python。修改分类内容时编辑生成器并重新运行；生成资源只用于本地测试站：

```bash
# 在 target_web/ 中执行
python3 scripts/generate-link-pages.py
go run scripts/generate-fixtures.go
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' \
  -o frontend/fixtures/downloads/sample.exe scripts/sample-download/main.go
./build.sh --frontend-only
docker compose up -d --no-build --no-deps --wait frontend
```

## HTTPS 证书

生成方式参考 `FutureConsole/deploy/lib/cert.sh` 的 `gen_self_signed`：ECDSA P-256 显式命名曲线、有效期 825 天；SAN 包含指定宿主机 IPv4、`localhost`、`127.0.0.1` 和 `TARGET_WEB_EXTERNAL_HOST`；设置 `CA:FALSE`、`digitalSignature`、`serverAuth`。

证书文件为 `certs/fullchain.pem` 和 `certs/privkey.pem`。私钥权限 600，仅只读挂载到前端容器 `/etc/nginx/certs`，不进入任一镜像。Nginx 在启动时加载并校验证书。每次 `up` 检查证书，缺失、损坏、密钥不配对、IP/外站主机名不匹配或有效期不足 30 天时重新生成；否则复用。`up` 重建容器以载入证书，`down` 保留证书。

自签名证书默认不受浏览器信任。使用 `--cacert` 可校验证书和访问地址：

```bash
curl --noproxy '*' http://192.168.5.10:38080/healthz
curl --noproxy '*' --cacert certs/fullchain.pem https://192.168.5.10:38443/healthz
curl --noproxy '*' --cacert certs/fullchain.pem https://127.0.0.1:38443/healthz
```

健康响应示例：`{"status":"ok","service":"target_web","scheme":"https"}`。`scheme` 表示浏览器到 Nginx 使用的协议；后端依赖 Nginx 覆盖的 `X-Forwarded-Proto`，因此后端应保持不发布宿主机端口。

更换 IP 时执行 `up --host <新 IP>`；手动续签：

```bash
./target_web.sh cert --host 192.168.5.10
./target_web.sh up --host 192.168.5.10
```

## 管理与开发验证

```bash
./target_web.sh status
./target_web.sh logs
./target_web.sh down

# 修改单个服务后，独立重建；首次启动仍使用上面的启动脚本
./build.sh --frontend-only
docker compose up -d --no-build --no-deps --wait frontend
./build.sh --backend-only
docker compose up -d --no-build --no-deps --wait backend
```

仅运行本地 Go API（需 Go 1.26.x，无需证书、静态文件）：

```bash
cd backend
go run ./cmd/server -http-addr 127.0.0.1:8000
# 另一个终端访问 http://127.0.0.1:8000/api
```

本地后端不提供页面。完整前后端联调使用 Compose，静态文件修改后重建 `frontend`。

```bash
# target_web/backend 中
go test -race ./...
go vet ./...
go build ./...

# 仓库根目录中
python3 -m unittest discover -s target_web/tests -v
```

协议参考 [HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110.html)、[Go net/http](https://pkg.go.dev/net/http)；代理配置参考 [Nginx upstream](https://nginx.org/en/docs/http/ngx_http_upstream_module.html) 和 [Nginx proxy](https://nginx.org/en/docs/http/ngx_http_proxy_module.html)。
