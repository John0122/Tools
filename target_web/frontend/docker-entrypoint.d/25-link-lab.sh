#!/bin/sh
# 启动时生成站点配置；支持复用镜像在不同测试域名下部署。
set -eu
external=${TARGET_WEB_EXTERNAL_HOST:-pz-target-web.com}
printf '%s\n' "$external" | awk '
  length($0)>253 || $0 !~ /\./ || $0 ~ /^[0-9.]+$/ {exit 1}
  {n=split($0,a,"."); for(i=1;i<=n;i++) if(length(a[i])<1 || length(a[i])>63 || a[i] !~ /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/) exit 1}
' || { echo '外站主机名格式错误' >&2; exit 1; }
sed "s/__LAB_EXTERNAL_HOST__/$external/g" /etc/nginx/external.conf.template > /etc/nginx/conf.d/external.conf
# 只处理明确的文本产物目录，二进制资源不参与替换。
find /usr/share/nginx/html/links /usr/share/nginx/html/fixtures /usr/share/nginx/html/js \
  -type f \( -name '*.html' -o -name '*.css' -o -name '*.js' -o -name '*.json' \) \
  -exec sed -i "s/__LAB_EXTERNAL_HOST__/$external/g" {} +
