# 校验单个 IPv4 HTTP(S) 入口；只接受地址、可选端口和可选末尾斜杠。
# 范围依据：https://www.iana.org/assignments/iana-ipv4-special-registry/
function private_ip(a) {
    return a[1] == 10 || (a[1] == 172 && a[2] >= 16 && a[2] <= 31) || \
        (a[1] == 192 && a[2] == 168)
}

function public_ip(a) {
    # 先排除非公网单播范围；192.0.0.9/10 是 IANA 明确列出的公网例外。
    if (private_ip(a) || a[1] == 0 || a[1] == 127 || a[1] >= 224) return 0
    if (a[1] == 100 && a[2] >= 64 && a[2] <= 127) return 0
    if (a[1] == 169 && a[2] == 254) return 0
    if (a[1] == 192 && a[2] == 0 && a[3] == 0) return a[4] == 9 || a[4] == 10
    if (a[1] == 192 && ((a[2] == 0 && a[3] == 2) || (a[2] == 88 && a[3] == 99))) return 0
    if (a[1] == 198 && (a[2] == 18 || a[2] == 19 || (a[2] == 51 && a[3] == 100))) return 0
    if (a[1] == 203 && a[2] == 0 && a[3] == 113) return 0
    return 1
}

{
    # 严格字符集也保证入口能安全写入 HTML、JSON 和 sed 替换表达式。
    if (NR != 1 || $0 !~ /^https?:\/\/[0-9.]+(:[0-9]+)?\/?$/) exit 1
    authority = $0
    sub(/^https?:\/\//, "", authority)
    sub(/\/$/, "", authority)
    parts = split(authority, endpoint, ":")
    if (split(endpoint[1], octets, ".") != 4) exit 1
    for (i = 1; i <= 4; i++) {
        if (octets[i] !~ /^[0-9]+$/ || length(octets[i]) > 3 || octets[i] + 0 > 255) exit 1
        if (length(octets[i]) > 1 && substr(octets[i], 1, 1) == "0") exit 1
    }
    if (parts == 2 && (length(endpoint[2]) > 5 || endpoint[2] + 0 < 1 || \
        endpoint[2] + 0 > 65535 || substr(endpoint[2], 1, 1) == "0")) exit 1
    if (group == "internal") {
        # 本站字段是裸 IP，http:// 仅由调用方临时添加；不得接受端口或末尾斜杠。
        if ($0 != "http://" endpoint[1]) exit 1
        next
    }
    # 本站 IP 始终归入内站，不因端口或协议不同重复出现在另外两组。
    if (endpoint[1] == main_ip) exit 1
    if (group == "public" && public_ip(octets)) next
    if (group == "lan" && private_ip(octets)) next
    exit 1
}
