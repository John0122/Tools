#!/usr/bin/env bash
# 构建并导出双镜像离线包，只携带目标机器运行所需的白名单文件。
set -euo pipefail

ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
VERSION=${TARGET_WEB_IMAGE_TAG:-local}
ARCH=""
OUTPUT="$ROOT/dist"
NO_BUILD=false

die() { printf '错误：%s\n' "$*" >&2; exit 1; }
usage() {
  cat <<'HELP'
用法：./package.sh [--version <标签>] [--arch amd64|arm64] [--output <目录>] [--no-build]

默认复用 build.sh 构建前后端镜像，再生成可复制到目标机器的离线 tar.gz 包。
--version   两个镜像的标签，默认 TARGET_WEB_IMAGE_TAG 或 local
--arch      Linux 目标架构，默认与当前 Docker 服务一致
--output    压缩包输出目录，默认 target_web/dist；拒绝覆盖同名文件
--no-build  只导出本地已有镜像，不构建、不拉取，无需 Go

包中包含 deploy.sh、运行配置、两个镜像及 SHA-256 校验清单；不包含本机证书和源码。
HELP
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version|--arch|--output)
      [[ $# -ge 2 && -n "$2" && "$2" != --* ]] || die "$1 需要参数"
      case "$1" in
        --version) VERSION=$2 ;;
        --arch) ARCH=$2 ;;
        --output) OUTPUT=$2 ;;
      esac
      shift 2 ;;
    --no-build) NO_BUILD=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "未知参数：$1（使用 --help 查看用法）" ;;
  esac
done
[[ "$VERSION" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]] || die "镜像标签格式无效"
[[ -z "$ARCH" || "$ARCH" == amd64 || "$ARCH" == arm64 ]] || die "目标架构必须为 amd64 或 arm64"
for tool in docker tar; do command -v "$tool" >/dev/null 2>&1 || die "缺少命令：$tool"; done
if command -v sha256sum >/dev/null 2>&1; then
  SHA256=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  SHA256=(shasum -a 256)
else
  die "需要 sha256sum 或 shasum 生成校验清单"
fi
docker info >/dev/null 2>&1 || die "无法连接 Docker 服务"
if [[ -z "$ARCH" ]]; then
  actual_arch=$(docker info --format '{{.Architecture}}')
  case "$actual_arch" in
    amd64|x86_64) ARCH=amd64 ;;
    arm64|aarch64) ARCH=arm64 ;;
    *) die "不支持的 Docker 架构：$actual_arch" ;;
  esac
fi

mkdir -p -- "$OUTPUT"
OUTPUT=$(cd -- "$OUTPUT" && pwd)
NAME="target-web-$VERSION-linux-$ARCH"
ARCHIVE="$OUTPUT/$NAME.tar.gz"
[[ ! -e "$ARCHIVE" && ! -L "$ARCHIVE" ]] || die "目标包已存在：$ARCHIVE，请更换标签或输出目录"
# 临时文件与最终包处于同一文件系统，完成后才发布；异常退出只清理本次目录。
STAGING=$(mktemp -d "$OUTPUT/.package.XXXXXX")
trap 'rm -rf -- "$STAGING"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
BUNDLE="$STAGING/$NAME"
mkdir -p "$BUNDLE"

if [[ "$NO_BUILD" == false ]]; then
  bash "$ROOT/build.sh" --version "$VERSION" --arch "$ARCH"
fi

# 同时记录镜像 ID 与平台，部署时拒绝错架构或被其他导入覆盖的标签。
IMAGE_IDS=()
for service in frontend backend; do
  ref="target-web-$service:$VERSION"
  # 多平台存储按目标架构检查；旧版 Docker 回退后仍须精确匹配平台。
  if ! details=$(docker image inspect --platform "linux/$ARCH" --format '{{.Os}}/{{.Architecture}} {{.Id}}' "$ref" 2>/dev/null); then
    details=$(docker image inspect --format '{{.Os}}/{{.Architecture}} {{.Id}}' "$ref") || die "本地镜像不存在：$ref"
  fi
  read -r platform image_id <<< "$details"
  [[ "$platform" == "linux/$ARCH" ]] || die "$ref 的平台为 $platform，需要 linux/$ARCH"
  [[ "$image_id" =~ ^sha256:[a-f0-9]{64}$ ]] || die "无法读取镜像 ID：$ref"
  IMAGE_IDS+=("$image_id")
done

# 明确列举运行文件，包含宿主机使用的 IP 与域名校验器，避免遗漏离线依赖或带入私钥。
FILES=(compose.yaml deploy.sh target_web.sh README.md scripts/generate-cert.sh
  frontend/docker-entrypoint.d/ip-origin.awk frontend/docker-entrypoint.d/external-hosts.awk)
for file in "${FILES[@]}"; do
  mkdir -p "$BUNDLE/$(dirname -- "$file")"
  cp "$ROOT/$file" "$BUNDLE/$file"
done
chmod 755 "$BUNDLE/deploy.sh" "$BUNDLE/target_web.sh" "$BUNDLE/scripts/generate-cert.sh"
printf 'format=1\nimage_tag=%s\nimage_arch=%s\nfrontend_id=%s\nbackend_id=%s\n' \
  "$VERSION" "$ARCH" "${IMAGE_IDS[0]}" "${IMAGE_IDS[1]}" > "$BUNDLE/bundle.env"
printf '导出前后端镜像：标签 %s，linux/%s\n' "$VERSION" "$ARCH"
# 保留兼容旧版 Docker 的导出方式；多平台镜像可能附带其他已缓存的平台。
docker image save --output "$BUNDLE/images.tar" "target-web-frontend:$VERSION" "target-web-backend:$VERSION"
(
  cd "$BUNDLE"
  "${SHA256[@]}" "${FILES[@]}" bundle.env images.tar > SHA256SUMS
)
# macOS 不将扩展属性写入 tar；目录名与文件名不依赖目标机器路径。
COPYFILE_DISABLE=1 tar -czf "$STAGING/$NAME.tar.gz" -C "$STAGING" "$NAME"
# 使用不覆盖的硬链接原子发布，避免并发打包时覆盖另一个已完成的产物。
ln "$STAGING/$NAME.tar.gz" "$ARCHIVE" || die "无法发布压缩包，目标可能已存在：$ARCHIVE"
printf '已生成离线包：%s\n解压后进入 %s，执行 ./deploy.sh。\n' "$ARCHIVE" "$NAME"
