#!/usr/bin/env bash
# 在宿主机检测访问地址并准备证书，再由 Docker Compose 管理 Web 靶机。
set -euo pipefail

ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)

die() { printf '错误：%s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
用法：./target_web.sh [up|down|status|logs|cert] [--host <宿主机 IPv4>] [--no-build]

  up      生成或复用证书，构建并启动容器（默认命令）
  down    停止并移除容器，保留证书
  status  查看容器状态
  logs    持续查看服务日志
  cert    重新生成证书；随后执行 up 使运行中的服务载入新证书

up/cert 默认检测宿主机默认路由的 IPv4；多网卡时可用 --host 指定。
--no-build 仅用于 up，直接使用本地已有的两个镜像；完整构建需要本机 Go。
HTTP：38080；HTTPS：38443；后端内部 API：8000。
EOF
}

# 参考 FutureConsole：检测宿主机默认路由，不使用容器地址或 Docker 网桥地址。
detect_host() {
  local iface address=""
  case "$(uname -s)" in
    Darwin)
      iface=$(route -n get default 2>/dev/null | awk '/interface:/{print $2; exit}') || iface=""
      if [[ -n "$iface" ]]; then
        address=$(ipconfig getifaddr "$iface" 2>/dev/null) || address=""
      fi
      ;;
    Linux)
      if command -v ip >/dev/null 2>&1; then
        address=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i=1; i<=NF; i++) if ($i=="src") {print $(i+1); exit}}') || address=""
      fi
      ;;
  esac
  printf '%s' "$address"
}

compose() {
  docker compose --project-directory "$ROOT" -f "$ROOT/compose.yaml" "$@"
}

command=up
host=""
no_build=false
if [[ $# -gt 0 && "$1" != -* ]]; then
  command=$1
  shift
fi
while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)
      [[ $# -ge 2 && -n "$2" ]] || die "--host 后需要提供 IPv4 地址"
      host=$2
      shift 2
      ;;
    --no-build) no_build=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "未知参数：$1（使用 --help 查看用法）" ;;
  esac
done
case "$command" in
  up|cert) ;;
  down|status|logs) [[ -z "$host" ]] || die "--host 仅用于 up/cert" ;;
  *) die "未知命令：$command（使用 --help 查看用法）" ;;
esac

[[ "$no_build" == false || "$command" == up ]] || die "--no-build 仅用于 up"

if [[ "$command" != cert ]]; then
  command -v docker >/dev/null 2>&1 || die "请先安装 Docker 和 Docker Compose v2"
  docker compose version >/dev/null 2>&1 || die "需要 Docker Compose v2"
  docker info >/dev/null 2>&1 || die "无法连接 Docker，请先启动 Docker 服务"
fi

case "$command" in
  up|cert)
    [[ -n "$host" ]] || host=$(detect_host)
    [[ -n "$host" ]] || die "无法检测宿主机 IPv4，请使用 --host <本地 IP> 指定"
    # Compose 同步注入前端 IP 样例与后端允许来源，外站域名也交给证书脚本校验。
    export TARGET_WEB_HOST="$host"
    export TARGET_WEB_EXTERNAL_HOST="${TARGET_WEB_EXTERNAL_HOST:-pz-target-web.com}"
    if [[ "$command" == cert ]]; then
      bash "$ROOT/scripts/generate-cert.sh" "$host" "$ROOT/certs" --force
      printf '执行 ./target_web.sh up --host %s 使服务载入新证书。\n' "$host"
      exit 0
    fi
    bash "$ROOT/scripts/generate-cert.sh" "$host" "$ROOT/certs"
    if [[ "$no_build" == false ]]; then
      bash "$ROOT/build.sh"
    fi
    # 重建确保 IP 变化或证书续签后载入新文件，并等待健康检查通过。
    # 移除本 Compose 项目旧版 web 服务，释放端口供独立 frontend 使用。
    compose up -d --no-build --force-recreate --remove-orphans --wait --wait-timeout 60
    printf '\nHTTP： http://%s:38080\nHTTPS：https://%s:38443\n' "$host" "$host"
    printf '外链测试目录：http://%s:38080/links/\n' "$host"
    printf '外站需在浏览器和扫描器所在主机解析：%s -> %s\n' "$TARGET_WEB_EXTERNAL_HOST" "$host"
    printf '自签名证书会触发浏览器信任提示；可使用以下命令校验：\n'
    printf "curl --noproxy '*' --cacert %q %q\n" "$ROOT/certs/fullchain.pem" "https://$host:38443/healthz"
    ;;
  down) compose down ;;
  status) compose ps ;;
  logs) compose logs --follow --tail=100 ;;
esac
