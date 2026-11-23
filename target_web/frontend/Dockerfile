# 参考 FutureConsole：仅复制已整理的静态产物和 Nginx 配置，无 RUN，不需要跨架构模拟器。
# 构建上下文由 ../build.sh 临时生成，原生 HTML/JS/CSS 无需 Node.js 编译。
FROM nginx:1.31-alpine-slim
ARG VERSION=local
ARG COMMIT=
LABEL org.opencontainers.image.title="target-web-frontend" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}"
COPY nginx/default.conf /etc/nginx/conf.d/default.conf
COPY nginx/proxy-api.conf /etc/nginx/proxy-api.conf
COPY nginx/lab-locations.conf /etc/nginx/lab-locations.conf
COPY nginx/external.conf.template /etc/nginx/external.conf.template
COPY --chmod=755 entrypoint/25-link-lab.sh /docker-entrypoint.d/25-link-lab.sh
COPY html/ /usr/share/nginx/html/
EXPOSE 8080 8443
