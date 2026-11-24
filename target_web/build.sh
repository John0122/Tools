#!/usr/bin/env bash
# 参考 FutureConsole/deploy/build.sh：本机编译、独立产物上下文、基础镜像按架构复用。
# Dockerfile 只组装运行镜像；兼容 macOS Bash 3.2，不依赖 Node.js 或 pnpm。
set -euo pipefail

ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
VERSION=${TARGET_WEB_IMAGE_TAG:-local}
ARCH=""
MODE=all
PRINT_BASE_IMAGES=false
CONTEXT_DIR=""

die() { printf '错误：%s\n' "$*" >&2; exit 1; }
info() { printf '%s\n' "$*"; }
usage() {
  cat <<'HELP'
用法：./build.sh [--version <标签>] [--arch amd64|arm64] [--frontend-only|--backend-only] [--base-images]

默认在本机交叉编译 Go，整理原生前端文件，再构建两个独立运行镜像。
--version        镜像标签，默认 TARGET_WEB_IMAGE_TAG 或 local
--arch           Linux 目标架构，默认与当前 Docker 服务一致
--frontend-only  只构建前端，不需要 Go
--backend-only   只编译并构建后端
--base-images    仅输出所选服务的基础镜像标签，不检查环境或构建
HELP
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version|--arch)
      [[ $# -ge 2 && -n "$2" && "$2" != --* ]] || die "$1 需要参数"
      if [[ "$1" == --version ]]; then VERSION=$2; else ARCH=$2; fi
      shift 2 ;;
    --frontend-only|--backend-only)
      [[ "$MODE" == all ]] || die "只能指定一种单服务构建模式"
      if [[ "$1" == --frontend-only ]]; then MODE=frontend; else MODE=backend; fi
      shift ;;
    --base-images) PRINT_BASE_IMAGES=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "未知参数：$1" ;;
  esac
done
[[ "$VERSION" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]] || die "镜像标签格式无效"
[[ -z "$ARCH" || "$ARCH" == amd64 || "$ARCH" == arm64 ]] || die "目标架构必须为 amd64 或 arm64"
SERVICES=(backend frontend)
if [[ "$MODE" != all ]]; then SERVICES=("$MODE"); fi

# 基础镜像版本仅在 Dockerfile 中维护，跳过 scratch 和前面已声明的阶段。
base_images() {
  local service
  for service in "${SERVICES[@]}"; do
    awk '
      toupper($1) == "FROM" {
        for (i = 2; i <= NF; i++) {
          if ($i ~ /^--/) continue
          ref = $i
          if (tolower(ref) != "scratch" && !(tolower(ref) in stages)) print ref
          if (toupper($(i + 1)) == "AS") stages[tolower($(i + 2))] = 1
          break
        }
      }
    ' "$ROOT/$service/Dockerfile"
  done | awk '!seen[$0]++'
}
if [[ "$PRINT_BASE_IMAGES" == true ]]; then base_images; exit 0; fi

command -v docker >/dev/null 2>&1 || die "请先安装 Docker"
docker info >/dev/null 2>&1 || die "无法连接 Docker 服务"
if [[ -z "$ARCH" ]]; then
  actual_arch=$(docker info --format '{{.Architecture}}')
  case "$actual_arch" in
    amd64|x86_64) ARCH=amd64 ;;
    arm64|aarch64) ARCH=arm64 ;;
    *) die "不支持的 Docker 架构：${actual_arch}" ;;
  esac
fi
if [[ "$MODE" != frontend ]]; then
  command -v go >/dev/null 2>&1 || die "本机构建后端需要 Go，版本以 backend/go.mod 为准"
  want=$(awk '$1 == "go" {split($2,v,"."); print v[1] "." v[2]; exit}' "$ROOT/backend/go.mod")
  have=$(cd "$ROOT/backend" && go env GOVERSION)
  [[ "$have" == "go${want}" || "$have" == "go${want}."* ]] || die "Go 版本为 ${have}，需要 ${want}.x"
fi

# 与 FutureConsole 一致：检查目标平台的本地缓存，仅缺失时拉取，失败立即终止。
base_image_available() {
  local ref=$1 actual
  if ! actual=$(docker image inspect --platform "linux/$ARCH" --format '{{.Os}}/{{.Architecture}}' "$ref" 2>/dev/null); then
    actual=$(docker image inspect --format '{{.Os}}/{{.Architecture}}' "$ref" 2>/dev/null) || return 1
  fi
  [[ "$actual" == "linux/$ARCH" ]]
}
refs=$(base_images)
while IFS= read -r ref; do
  [[ -n "$ref" ]] || continue
  if base_image_available "$ref"; then
    info "复用基础镜像：${ref}（linux/${ARCH}）"
  else
    docker pull --platform "linux/$ARCH" "$ref" || die "拉取基础镜像失败：${ref}"
    base_image_available "$ref" || die "基础镜像与目标架构不匹配：${ref}"
  fi
done <<<"$refs"

# 每次构建独占临时上下文；无论成功或失败都清理，避免混入旧产物或其他服务源码。
mkdir -p "$ROOT/build"
CONTEXT_DIR=$(mktemp -d "$ROOT/build/context.XXXXXX")
trap 'rm -rf -- "$CONTEXT_DIR"' EXIT
COMMIT=$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || true)

prepare_backend() {
  info "编译后端：linux/${ARCH}"
  mkdir -p "$CONTEXT_DIR/backend"
  (
    cd "$ROOT/backend"
    CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "$CONTEXT_DIR/backend/server" ./cmd/server
  )
}
prepare_frontend() {
  info "整理原生前端静态文件"
  mkdir -p "$CONTEXT_DIR/frontend/html" "$CONTEXT_DIR/frontend/nginx" "$CONTEXT_DIR/frontend/entrypoint"
  cp "$ROOT/frontend/index.html" "$CONTEXT_DIR/frontend/html/"
  cp -R "$ROOT/frontend/css" "$ROOT/frontend/js" "$ROOT/frontend/links" "$ROOT/frontend/fixtures" "$CONTEXT_DIR/frontend/html/"
  cp "$ROOT/frontend/nginx/"*.conf "$ROOT/frontend/nginx/"*.template "$CONTEXT_DIR/frontend/nginx/"
  # 启动脚本、IP 与域名校验器共同进入运行镜像，均不公开为静态资源。
  cp "$ROOT/frontend/docker-entrypoint.d/25-link-lab.sh" \
    "$ROOT/frontend/docker-entrypoint.d/ip-origin.awk" \
    "$ROOT/frontend/docker-entrypoint.d/external-hosts.awk" "$CONTEXT_DIR/frontend/entrypoint/"
}

# 两个服务先准备产物，再分别组装镜像；源码、证书、测试与规范文件不进入构建上下文。
for service in "${SERVICES[@]}"; do "prepare_$service"; done
for service in "${SERVICES[@]}"; do
  (
    # 避免环境变量选择另一份镜像存储，复用当前 Docker context 的默认 builder。
    unset BUILDX_BUILDER
    docker build --pull=false --platform "linux/$ARCH" \
      --build-arg "VERSION=$VERSION" --build-arg "COMMIT=$COMMIT" \
      -f "$ROOT/$service/Dockerfile" -t "target-web-$service:$VERSION" "$CONTEXT_DIR/$service"
  )
  info "已构建：target-web-${service}:${VERSION}"
done
