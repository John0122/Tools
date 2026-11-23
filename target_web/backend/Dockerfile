# 参考 FutureConsole：本机先交叉编译，镜像只复制产物与 CA，不包含源码和 Go 工具链。
# 构建上下文由 ../build.sh 临时生成，不直接使用 backend 源码目录。
FROM alpine:3.24 AS certs
FROM scratch
ARG VERSION=local
ARG COMMIT=
LABEL org.opencontainers.image.title="target-web-backend" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}"
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY server /server
USER 65532:65532
EXPOSE 8000
ENTRYPOINT ["/server"]
CMD ["-http-addr", ":8000"]
