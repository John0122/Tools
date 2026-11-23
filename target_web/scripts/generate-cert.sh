#!/usr/bin/env bash
# 参考 FutureConsole/deploy/lib/cert.sh，在宿主机生成带访问 IP 的自签名证书。
set -euo pipefail

die() { printf '错误：%s\n' "$*" >&2; exit 1; }

if [[ $# -lt 2 || $# -gt 3 || ( $# -eq 3 && "$3" != --force ) ]]; then
  die "用法：$0 <宿主机 IPv4> <证书目录> [--force]"
fi
host=$1
cert_dir=$2
force=${3:-}
external_host=${TARGET_WEB_EXTERNAL_HOST:-pz-target-web.com}

# DNS 名称也进入 OpenSSL SAN 参数；逐标签验证，拒绝通配符、URL 和注入字符。
valid_external_host() {
  local label
  local -a labels
  [[ ${#1} -le 253 && "$1" == *.* && "$1" != *..* && "$1" != *. ]] || return 1
  [[ ! "$1" =~ ^[0-9.]+$ ]] || return 1
  IFS=. read -r -a labels <<< "$1"
  for label in "${labels[@]}"; do
    [[ ${#label} -le 63 && "$label" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$ ]] || return 1
  done
}
valid_external_host "$external_host" || die "外站主机名无效：$external_host"

# 仅接受无前导零的 IPv4，避免把 URL、换行或 OpenSSL 扩展混入证书参数。
valid_ipv4() {
  local octet
  local -a octets
  [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] || return 1
  IFS=. read -r -a octets <<< "$1"
  for octet in "${octets[@]}"; do
    [[ ${#octet} -eq 1 || "$octet" != 0* ]] || return 1
    (( 10#$octet <= 255 )) || return 1
  done
}

valid_ipv4 "$host" || die "不是有效的 IPv4 地址：$host"
command -v openssl >/dev/null 2>&1 || die "需要 OpenSSL 1.1.1 或更新版本"
[[ -n "$cert_dir" ]] || die "证书目录不能为空"
umask 077
mkdir -p -- "$cert_dir"
cert_dir=$(cd -- "$cert_dir" && pwd)
cert="$cert_dir/fullchain.pem"
key="$cert_dir/privkey.pem"

# 复用前同时验证有效期、访问地址、回环地址及密钥配对，避免 Web 服务启动失败。
reusable_cert() {
  local cert_pub key_pub
  [[ -s "$cert" && -s "$key" ]] || return 1
  openssl x509 -in "$cert" -noout -checkend 2592000 >/dev/null 2>&1 || return 1
  openssl verify -CAfile "$cert" -purpose sslserver -verify_ip "$host" "$cert" >/dev/null 2>&1 || return 1
  openssl verify -CAfile "$cert" -verify_ip 127.0.0.1 "$cert" >/dev/null 2>&1 || return 1
  openssl verify -CAfile "$cert" -verify_hostname localhost "$cert" >/dev/null 2>&1 || return 1
  openssl verify -CAfile "$cert" -verify_hostname "$external_host" "$cert" >/dev/null 2>&1 || return 1
  cert_pub=$(openssl x509 -in "$cert" -noout -pubkey 2>/dev/null) || return 1
  key_pub=$(openssl pkey -in "$key" -passin pass: -pubout 2>/dev/null) || return 1
  [[ "$cert_pub" == "$key_pub" ]]
}

if [[ "$force" != --force ]] && reusable_cert; then
  chmod 600 "$key"
  printf '复用 HTTPS 证书：%s\n' "$cert"
  exit 0
fi

san="IP:$host,DNS:localhost,DNS:$external_host"
[[ "$host" == 127.0.0.1 ]] || san="$san,IP:127.0.0.1"
temporary=$(mktemp -d "$cert_dir/.generate.XXXXXX")
trap 'rm -rf -- "$temporary"' EXIT

# 与 FutureConsole 一致：P-256、显式命名曲线、825 天、仅用于服务器签名。
# 先完整生成到临时目录；OpenSSL 失败时保留原证书和私钥。
openssl req -x509 -newkey ec \
  -pkeyopt ec_paramgen_curve:prime256v1 -pkeyopt ec_param_enc:named_curve \
  -nodes -days 825 -subj "/CN=$host" \
  -addext "subjectAltName=$san" \
  -addext "basicConstraints=critical,CA:FALSE" \
  -addext "keyUsage=critical,digitalSignature" \
  -addext "extendedKeyUsage=serverAuth" \
  -keyout "$temporary/privkey.pem" -out "$temporary/fullchain.pem" \
  || die "生成证书失败，请检查 OpenSSL 版本及目录权限"
chmod 600 "$temporary/privkey.pem"
chmod 644 "$temporary/fullchain.pem"
mv -f -- "$temporary/privkey.pem" "$key"
mv -f -- "$temporary/fullchain.pem" "$cert"
printf '已生成 HTTPS 证书（825 天，SAN：%s）\n' "$san"
