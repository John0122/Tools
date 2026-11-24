# 宿主机与容器共用域名列表校验；逗号分隔，允许两侧空格或制表符。
# 完整校验后才输出去重的空格分隔列表，供 Nginx、证书和启动提示使用。
function reject() { invalid = 1; exit 1 }
NR != 1 { reject() }
{
    count = split($0, hosts, ",")
    for (i = 1; i <= count; i++) {
        host = hosts[i]
        gsub(/^[ \t]+|[ \t]+$/, "", host)
        if (length(host) > 253 || host !~ /\./ || host ~ /^[0-9.]+$/) reject()
        labels_count = split(host, labels, ".")
        for (j = 1; j <= labels_count; j++) {
            if (length(labels[j]) < 1 || length(labels[j]) > 63 ||
                labels[j] !~ /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/) reject()
        }
        if (!seen[host]++) result = result (result == "" ? "" : " ") host
    }
}
END {
    if (invalid || NR != 1 || result == "") exit 1
    print result
}
