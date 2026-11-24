#!/bin/sh
# 启动时生成站点配置；支持复用镜像在不同测试域名和 IP 下部署。
set -eu
# 单个或多个域名统一读取列表；未配置时使用三个默认域名，第一个域名用于外链样例。
external_hosts=$(printf '%s\n' "${TARGET_WEB_EXTERNAL_HOSTS:-www.pz-target-web.com,www.pz-target-web-1.com,www.pz-target-web-2.com}" |
  awk -f /docker-entrypoint.d/external-hosts.awk) || { echo '域名列表须为逗号分隔的小写 DNS 名称' >&2; exit 1; }
external=${external_hosts%% *}
main_ip=${TARGET_WEB_HOST:-127.0.0.1}
lan_origin=${TARGET_WEB_LAN_IP_ORIGIN:-}

# 配置校验发生在写文件前；拒绝域名、特殊地址及重复本站 IP，保持三个分组含义稳定。
validate_origin() {
  printf '%s\n' "$1" | awk -v group="$2" -v main_ip="$main_ip" \
    -f /docker-entrypoint.d/ip-origin.awk
}
validate_origin "http://$main_ip" internal || { echo '本站 IPv4 地址格式错误' >&2; exit 1; }
if [ -n "$lan_origin" ]; then
  validate_origin "$lan_origin" lan || { echo '局域网 IP 入口须为其他设备的 http(s)://私有IPv4[:端口]' >&2; exit 1; }
fi

# 局域网样例由服务端生成，未配置时只显示说明；公网完整路径保留在静态页面中。
ip_sample() {
  printf '<p><a href="%s" target="_blank" rel="noopener" data-case-id="ip-address-%s">访问%s</a></p><p>目标地址 <code>%s</code></p>' "$1" "$2" "$3" "$1"
}
lan_sample='<p>尚未配置局域网 IP 入口。配置后可访问其他设备的样例。</p>'
lan_json=null
if [ -n "$lan_origin" ]; then
  lan_url="${lan_origin%/}/"
  lan_sample=$(ip_sample "$lan_url" lan '局域网设备')
  lan_json="\"$lan_url\""
fi
# 所有域名共用虚拟主机；静态样例的 URL 始终只替换为第一个域名。
sed "s/__LAB_EXTERNAL_HOSTS__/$external_hosts/g" /etc/nginx/external.conf.template > /etc/nginx/conf.d/external.conf
# 只处理明确的文本产物目录，二进制资源不参与替换。
find /usr/share/nginx/html/links /usr/share/nginx/html/fixtures /usr/share/nginx/html/js \
  -type f \( -name '*.html' -o -name '*.css' -o -name '*.js' -o -name '*.json' \) \
  -exec sed -i \
    -e "s|__LAB_EXTERNAL_HOST__|$external|g" \
    -e "s|__LAB_INTERNAL_HOST__|$main_ip|g" \
    -e "s|__LAB_LAN_IP_SAMPLE__|$lan_sample|g" \
    -e "s|\"__LAB_LAN_IP_URL__\"|$lan_json|g" {} +
