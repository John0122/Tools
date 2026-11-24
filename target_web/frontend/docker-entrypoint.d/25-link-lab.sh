#!/bin/sh
# 启动时生成站点配置；支持复用镜像在不同测试域名和 IP 下部署。
set -eu
external=${TARGET_WEB_EXTERNAL_HOST:-pz-target-web.com}
main_ip=${TARGET_WEB_HOST:-127.0.0.1}
public_origin=${TARGET_WEB_PUBLIC_IP_ORIGIN:-}
lan_origin=${TARGET_WEB_LAN_IP_ORIGIN:-}
printf '%s\n' "$external" | awk '
  length($0)>253 || $0 !~ /\./ || $0 ~ /^[0-9.]+$/ {exit 1}
  {n=split($0,a,"."); for(i=1;i<=n;i++) if(length(a[i])<1 || length(a[i])>63 || a[i] !~ /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/) exit 1}
' || { echo '外站主机名格式错误' >&2; exit 1; }

# 配置校验发生在写文件前；拒绝域名、特殊地址及重复本站 IP，保持三个分组含义稳定。
validate_origin() {
  printf '%s\n' "$1" | awk -v group="$2" -v main_ip="$main_ip" \
    -f /docker-entrypoint.d/ip-origin.awk
}
validate_origin "http://$main_ip" internal || { echo '本站 IPv4 地址格式错误' >&2; exit 1; }
if [ -n "$public_origin" ]; then
  validate_origin "$public_origin" public || { echo '公网 IP 入口须为其他站点的 http(s)://公网IPv4[:端口]' >&2; exit 1; }
fi
if [ -n "$lan_origin" ]; then
  validate_origin "$lan_origin" lan || { echo '局域网 IP 入口须为其他设备的 http(s)://私有IPv4[:端口]' >&2; exit 1; }
fi

# 可选样例由服务端生成，未配置时只显示说明，避免空 href 被误识别为本站链接。
ip_sample() {
  printf '<p><a href="%s" target="_blank" rel="noopener" data-case-id="ip-address-%s">访问%s</a></p><p>目标地址 <code>%s</code></p>' "$1" "$2" "$3" "$1"
}
public_sample='<p>尚未配置公网 IP 入口。配置后可访问公网外站样例。</p>'
lan_sample='<p>尚未配置局域网 IP 入口。配置后可访问其他设备的样例。</p>'
public_json=null
lan_json=null
if [ -n "$public_origin" ]; then
  public_url="${public_origin%/}/"
  public_sample=$(ip_sample "$public_url" public '公网外站')
  public_json="\"$public_url\""
fi
if [ -n "$lan_origin" ]; then
  lan_url="${lan_origin%/}/"
  lan_sample=$(ip_sample "$lan_url" lan '局域网设备')
  lan_json="\"$lan_url\""
fi
sed "s/__LAB_EXTERNAL_HOST__/$external/g" /etc/nginx/external.conf.template > /etc/nginx/conf.d/external.conf
# 只处理明确的文本产物目录，二进制资源不参与替换。
find /usr/share/nginx/html/links /usr/share/nginx/html/fixtures /usr/share/nginx/html/js \
  -type f \( -name '*.html' -o -name '*.css' -o -name '*.js' -o -name '*.json' \) \
  -exec sed -i \
    -e "s|__LAB_EXTERNAL_HOST__|$external|g" \
    -e "s|__LAB_INTERNAL_HOST__|$main_ip|g" \
    -e "s|__LAB_PUBLIC_IP_SAMPLE__|$public_sample|g" \
    -e "s|__LAB_LAN_IP_SAMPLE__|$lan_sample|g" \
    -e "s|\"__LAB_PUBLIC_IP_URL__\"|$public_json|g" \
    -e "s|\"__LAB_LAN_IP_URL__\"|$lan_json|g" {} +
