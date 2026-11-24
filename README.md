# Tools

工具集合仓库，各工具放在独立目录中，并提供对应的构建、运行和使用说明。

## 工具列表

| 工具 | 用途 | 技术栈 | 文档 |
| --- | --- | --- | --- |
| `target_web` | Web 靶机，提供九种 HTTP 方法与 13 类链接检测页面，包含 IP 地址分类、真实接口、资源、站内外暗链和死链 | 原生 HTML / JavaScript / CSS、Go、Nginx、Docker Compose | [使用说明](target_web/README.md) |

## 快速启动 target_web

支持 macOS、Linux。宿主机需安装 Docker、Docker Compose v2（支持 `up --wait`）、OpenSSL 1.1.1 或更新版本，以及 Go 1.26.x。原生前端无需 Node.js 或前端打包工具。

在仓库根目录执行：

```bash
cd target_web
./target_web.sh up

# 多网卡或需要指定证书中的本机 IPv4 时
./target_web.sh up --host 192.168.5.10
```

启动脚本自动检测本机 IPv4，生成或复用 HTTPS 证书，分别构建前后端镜像，并等待两个容器健康。

| 入口 | 地址或端口 | 说明 |
| --- | --- | --- |
| HTTPS | <https://127.0.0.1:38443> | 使用本机 IP 自签名证书，也可通过启动脚本输出的局域网地址访问 |
| 后端 | `8000` | 仅供 Docker 内部网络访问，由前端 Nginx 代理 |

前端仅发布 `38443:8443`；容器内 HTTP 8080 用于健康检查，不映射到宿主机。

自签名证书默认不受浏览器信任，证书配置与校验方式见 [HTTPS 证书说明](target_web/README.md#https-证书)。

页面支持 GET、HEAD、POST、PUT、PATCH、DELETE、OPTIONS、TRACE、CONNECT 九种方法，展示真实响应的状态码、耗时、响应头和正文。其中 TRACE、CONNECT 由后端向固定接口代发；CONNECT 返回 501 和方法说明，不建立隧道。

首页“外链检测”进入 `/links/` 二级目录，再访问 13 个分类页。分类页包含真实 HTML 标签、动态请求、GraphQL、WebSocket、表单与重定向，以及站内/站外隐藏链接和失效链接。“IP 地址链接”按内站、公网外站、局域网其他设备分组，地址配置见 [IP 地址链接配置](target_web/README.md#ip-地址链接配置)。其他分类的受控外站默认 `pz-target-web.com`，需要在访问设备的 DNS 或 hosts 中指向部署 IP，详见 [外链检测页面](target_web/README.md#外链检测页面)。

在 `target_web/` 目录中管理服务：

```bash
./target_web.sh status  # 查看容器状态
./target_web.sh logs    # 查看服务日志
./target_web.sh down    # 停止并移除容器，保留证书
```

## 构建与目录

`target_web` 的构建过程参考 FutureConsole：在宿主机交叉编译 Go，整理原生前端静态文件，再用独立产物上下文构建两个运行镜像。前端使用 Nginx，后端使用 `scratch`；本机证书与私钥不进入镜像。

```text
Tools/
├── README.md                 # 仓库概览与使用入口
├── AGENTS.md                 # 仓库协作规范
├── LICENSE                   # MIT 许可证
├── docs/                     # 设计与实施文档
└── target_web/
    ├── frontend/             # 原生前端、Nginx 配置与前端 Dockerfile
    ├── backend/              # Go API 服务与后端 Dockerfile
    ├── scripts/              # 证书、分类页面与演示资源生成脚本
    ├── build.sh              # 编译产物并构建前后端镜像
    ├── compose.yaml          # 双容器部署配置
    ├── target_web.sh         # 启动与管理入口
    └── README.md             # 完整使用说明
```

镜像标签、目标架构、独立构建和运行已有镜像的方法见 [镜像编译与构建](target_web/README.md#镜像编译与构建)。

协作规范沿用 FutureConsole，并按本仓库调整：[仓库规范](AGENTS.md)、[前端规范](target_web/frontend/AGENTS.md)、[后端规范](target_web/backend/AGENTS.md)。

## 许可证

本项目采用 [MIT License](LICENSE)。演示媒体、字体和照片另见 [素材来源与许可证](target_web/frontend/fixtures/SOURCES.md)。
