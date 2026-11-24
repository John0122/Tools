"""校验域名列表的解析边界；宿主机与前端容器共用此 AWK 校验器。"""

from pathlib import Path
import subprocess
import unittest


VALIDATOR = (Path(__file__).resolve().parents[1] / "frontend"
             / "docker-entrypoint.d" / "external-hosts.awk")


class ExternalHostsTests(unittest.TestCase):
    def parse(self, value):
        return subprocess.run(
            ["awk", "-f", str(VALIDATOR)], input=value + "\n",
            capture_output=True, text=True,
        )

    def test_valid_domains_keep_order_and_remove_duplicates(self):
        # 输出供 server_name 使用，但第一个域名还承担固定样例 URL 的语义。
        for value, expected in (
            ("www.pz-target-web.com", "www.pz-target-web.com"),
            (" first.lab.test,\tsecond.lab.test ,first.lab.test ", "first.lab.test second.lab.test"),
            ("a-b.lab.test,xn--bcher-kva.test", "a-b.lab.test xn--bcher-kva.test"),
        ):
            with self.subTest(value=value):
                result = self.parse(value)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout, expected + "\n")

    def test_invalid_items_reject_entire_list_without_partial_output(self):
        for item in ("", " ", "127.0.0.1", "999.1.1.1", "localhost",
                     "https://second.lab.test", "second.lab.test:38443", "*.lab.test",
                     "second..lab.test", "second.lab.test.", "-second.lab.test",
                     "second-.lab.test", "UPPER.lab.test", "second.lab.test;",
                     "second.lab.test\n", "second.lab.test\r", "second lab.test",
                     "second.lab.test/path", "second.lab.test#fragment",
                     "a" * 64 + ".test", ("a" * 63 + ".") * 4 + "test"):
            with self.subTest(item=item):
                result = self.parse("first.lab.test," + item)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")


if __name__ == "__main__":
    unittest.main()
