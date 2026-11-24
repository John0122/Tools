"""仅校验 IP 入口解析和分类，不启动服务或访问目标地址。"""

from pathlib import Path
import subprocess
import unittest


VALIDATOR = (Path(__file__).resolve().parents[1] / "frontend"
             / "docker-entrypoint.d" / "ip-origin.awk")


class IPOriginTests(unittest.TestCase):
    def accepts(self, origin, group, main_ip="192.168.5.10"):
        # 通过真实 AWK 入口检查退出码，确保测试覆盖部署时使用的校验逻辑。
        result = subprocess.run(
            ["awk", "-v", f"group={group}", "-v", f"main_ip={main_ip}",
             "-f", str(VALIDATOR)],
            input=origin + "\n", capture_output=True, text=True,
        )
        return result.returncode == 0

    def test_public_and_private_origins_keep_their_groups(self):
        for origin in ("https://1.1.1.1", "http://8.8.8.8:8080/",
                       "https://192.0.0.9", "https://192.0.0.10"):
            with self.subTest(origin=origin):
                self.assertTrue(self.accepts(origin, "public"))
                self.assertFalse(self.accepts(origin, "lan"))
        for origin in ("http://10.0.0.1", "http://172.16.0.1:80",
                       "https://172.31.255.254:65535/", "http://192.168.5.20:38080"):
            with self.subTest(origin=origin):
                self.assertTrue(self.accepts(origin, "lan"))
                self.assertFalse(self.accepts(origin, "public"))

    def test_non_public_special_ranges_are_rejected(self):
        for address in ("0.1.2.3", "100.64.0.1", "100.127.255.254",
                        "127.0.0.1", "169.254.1.1", "192.0.0.8",
                        "192.0.0.170", "192.0.2.1", "192.88.99.2",
                        "198.18.0.1", "198.19.255.254", "198.51.100.1",
                        "203.0.113.1", "224.0.0.1", "240.0.0.1", "255.255.255.255"):
            with self.subTest(address=address):
                self.assertFalse(self.accepts(f"https://{address}", "public"))
                self.assertFalse(self.accepts(f"https://{address}", "lan"))

    def test_current_site_ip_takes_precedence_over_other_groups(self):
        self.assertTrue(self.accepts("http://127.0.0.1", "internal"))
        self.assertFalse(self.accepts("http://192.168.5.10:8080", "lan"))
        self.assertFalse(self.accepts("https://1.1.1.1", "public", main_ip="1.1.1.1"))

    def test_deployment_host_must_be_a_bare_ipv4_address(self):
        # 与启动脚本一样拼接输入，防止校验通过后生成双端口或带路径的本站链接。
        self.assertTrue(self.accepts("http://192.168.5.10", "internal"))
        for host in ("192.168.5.10:8080", "192.168.5.10/", "192.168.5.10/path",
                     "192.168.5.10?x=1", "http://192.168.5.10", "192.168.05.10", ""):
            with self.subTest(host=host):
                self.assertFalse(self.accepts(f"http://{host}", "internal", main_ip=host))

    def test_only_literal_ipv4_origins_with_valid_ports_are_accepted(self):
        # 拒绝可改变 HTML/JSON 结构的输入，以及浏览器会歧义解析的地址形式。
        for origin in ("", "https://example.com", "https://[::1]", "//1.1.1.1",
                       "ftp://1.1.1.1", "http://user@1.1.1.1", "http://1.1.1.1/path",
                       "http://1.1.1.1?x=1", "http://1.1.1.1#part", "http://1.1.1.1:0",
                       "http://1.1.1.1:65536", "http://1.1.1.1:080", "http://1.1.1.1:",
                       "http://1.1.1.256", "http://01.1.1.1", "http://1.1.1",
                       "http://0x01010101", "http://16843009", " http://1.1.1.1",
                       "http://1.1.1.1\nhttp://8.8.8.8", 'http://1.1.1.1/"',
                       "http://1.1.1.1\\", "http://1.1.1.1|bad"):
            with self.subTest(origin=origin):
                self.assertFalse(self.accepts(origin, "public"))


if __name__ == "__main__":
    unittest.main()
