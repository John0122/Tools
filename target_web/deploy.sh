#!/usr/bin/env bash
# 在离线包目录校验并导入镜像，再复用现有地址、证书与 Compose 启动流程。
set -euo pipefail

ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
ARGS=()
HAS_ARGS=false
die() { printf '错误：%s\n' "$*" >&2; exit 1; }
usage() {
  cat <<'HELP'
用法：./deploy.sh [--host <宿主机 IPv4>] [--lan-ip <另一台设备 IPv4>]

在 package.sh 生成的离线包内执行：校验文件与架构，导入两个镜像，生成或复用证书并启动。
--host    指定本站访问 IP；省略时检测默认路由 IPv4
--lan-ip  指定另一套服务的私有 IPv4；省略时沿用已有环境配置或在终端提示输入

目标机器需要 Docker、Compose v2（支持 up --wait）、OpenSSL 和 sha256sum/shasum，无需 Go。
镜像标签使用包内元数据；TARGET_WEB_EXTERNAL_HOSTS 可配置单个域名或逗号分隔的多个域名。
TARGET_WEB_LAN_IP_ORIGIN 配置局域网样例。
服务管理：./target_web.sh status|logs|down；重新部署：./deploy.sh [参数]
HELP
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --host|--lan-ip)
      [[ $# -ge 2 && -n "$2" && "$2" != --* ]] || die "$1 需要 IPv4 参数"
      ARGS+=("$1" "$2"); HAS_ARGS=true; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "未知参数：$1（使用 --help 查看用法）" ;;
  esac
done

[[ -f "$ROOT/bundle.env" && -f "$ROOT/images.tar" && -f "$ROOT/SHA256SUMS" ]] || die "当前目录不是完整离线包；请先运行 package.sh，将产物解压后再部署"
if command -v sha256sum >/dev/null 2>&1; then
  SHA256=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  SHA256=(shasum -a 256)
else
  die "需要 sha256sum 或 shasum 校验离线包"
fi
printf '校验离线包完整性…\n'
(cd "$ROOT" && "${SHA256[@]}" -c SHA256SUMS) || die "校验失败，请重新传输或解压完整离线包"

# 元数据仅按固定键读取，不能 source/eval；重复、缺失或未知字段均拒绝。
FORMAT="" VERSION="" ARCH="" FRONTEND_ID="" BACKEND_ID=""
while IFS='=' read -r key value || [[ -n "$key" ]]; do
  case "$key" in
    format) [[ -z "$FORMAT" ]] || die "重复字段：$key"; FORMAT=$value ;;
    image_tag) [[ -z "$VERSION" ]] || die "重复字段：$key"; VERSION=$value ;;
    image_arch) [[ -z "$ARCH" ]] || die "重复字段：$key"; ARCH=$value ;;
    frontend_id) [[ -z "$FRONTEND_ID" ]] || die "重复字段：$key"; FRONTEND_ID=$value ;;
    backend_id) [[ -z "$BACKEND_ID" ]] || die "重复字段：$key"; BACKEND_ID=$value ;;
    *) die "未知元数据字段：$key" ;;
  esac
  [[ -n "$value" ]] || die "元数据字段不能为空：$key"
done < "$ROOT/bundle.env"
[[ "$FORMAT" == 1 ]] || die "不支持的离线包格式：$FORMAT"
[[ "$VERSION" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]] || die "离线包镜像标签无效"
[[ "$ARCH" == amd64 || "$ARCH" == arm64 ]] || die "离线包架构无效"
[[ "$FRONTEND_ID" =~ ^sha256:[a-f0-9]{64}$ && "$BACKEND_ID" =~ ^sha256:[a-f0-9]{64}$ ]] || die "离线包镜像 ID 无效"

for tool in docker openssl awk; do command -v "$tool" >/dev/null 2>&1 || die "缺少命令：$tool"; done
docker compose version >/dev/null 2>&1 || die "需要 Docker Compose v2"
docker info >/dev/null 2>&1 || die "无法连接 Docker 服务"
[[ "$(docker info --format '{{.OSType}}')" == linux ]] || die "请切换到 Linux 容器模式"
actual_arch=$(docker info --format '{{.Architecture}}')
case "$actual_arch" in
  amd64|x86_64) actual_arch=amd64 ;;
  arm64|aarch64) actual_arch=arm64 ;;
  *) die "不支持的 Docker 架构：$actual_arch" ;;
esac
[[ "$actual_arch" == "$ARCH" ]] || die "此包为 linux/$ARCH，目标 Docker 为 linux/$actual_arch；请重新打包匹配的架构"
export TARGET_WEB_IMAGE_TAG="$VERSION"
# 在变更本地镜像或容器前先校验 Compose 文件，禁止回退到联网拉取。
docker compose --project-directory "$ROOT" -f "$ROOT/compose.yaml" config --quiet
printf '导入前后端镜像：标签 %s，linux/%s\n' "$VERSION" "$ARCH"
docker image load --input "$ROOT/images.tar"
for service in frontend backend; do
  ref="target-web-$service:$VERSION"
  expected=$FRONTEND_ID
  if [[ "$service" == backend ]]; then expected=$BACKEND_ID; fi
  # 与打包端选择同一平台，避免多平台镜像在两台机器上取到不同的详情。
  if ! actual=$(docker image inspect --platform "linux/$ARCH" --format '{{.Os}}/{{.Architecture}} {{.Id}}' "$ref" 2>/dev/null); then
    actual=$(docker image inspect --format '{{.Os}}/{{.Architecture}} {{.Id}}' "$ref") || die "导入后镜像缺失：$ref"
  fi
  [[ "$actual" == "linux/$ARCH $expected" ]] || die "导入后镜像与包内记录不一致：$ref，未启动服务"
done

# Bash 3.2 在 nounset 下不接受空数组展开，分别处理有无地址参数。
if [[ "$HAS_ARGS" == true ]]; then
  exec bash "$ROOT/target_web.sh" up --no-build "${ARGS[@]}"
fi
exec bash "$ROOT/target_web.sh" up --no-build
