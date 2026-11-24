"""实际调用 OpenSSL，验证证书生命周期与访问地址，而非匹配脚本内容。"""

import os
from pathlib import Path
import ssl
import subprocess
import tempfile
import unittest


GENERATOR = Path(__file__).resolve().parents[1] / "scripts" / "generate-cert.sh"
DEFAULT_HOSTS = ("www.pz-target-web.com", "www.pz-target-web-1.com", "www.pz-target-web-2.com")


class CertificateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="target web cert ")
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name) / "certs"
        self.cert = self.directory / "fullchain.pem"
        self.key = self.directory / "privkey.pem"
        # 隔离部署环境，默认、单个和多个域名由每个用例显式指定。
        self.environment = dict(os.environ)
        self.environment.pop("TARGET_WEB_EXTERNAL_HOSTS", None)

    def generate(self, host="192.168.5.10", force=False, external_hosts=None):
        command = ["bash", str(GENERATOR), host, str(self.directory)]
        if force:
            command.append("--force")
        environment = dict(self.environment)
        if external_hosts is not None:
            environment["TARGET_WEB_EXTERNAL_HOSTS"] = external_hosts
        return subprocess.run(command, env=environment, capture_output=True, text=True)

    def openssl(self, *args):
        result = subprocess.run(
            ["openssl", *map(str, args)], capture_output=True, text=True
        )
        self.assertEqual(result.returncode, 0, result.stderr or result.stdout)
        return result.stdout

    def assert_generated(self, host="192.168.5.10", force=False):
        result = self.generate(host, force)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_certificate_matches_ip_and_loopback_with_server_usage(self):
        # SAN 缺失、曲线或用途错误将导致地址校验或握手失败。
        self.assert_generated()
        for host in ("192.168.5.10", "127.0.0.1"):
            self.openssl("verify", "-CAfile", self.cert, "-purpose", "sslserver",
                         "-verify_ip", host, self.cert)
        self.openssl("verify", "-CAfile", self.cert, "-verify_hostname",
                     "localhost", self.cert)
        details = self.openssl("x509", "-in", self.cert, "-text", "-noout")
        self.assertIn("ASN1 OID: prime256v1", details)
        self.assertIn("CA:FALSE", details)
        self.assertIn("Digital Signature", details)
        self.assertIn("TLS Web Server Authentication", details)
        self.openssl("x509", "-in", self.cert, "-checkend", 824 * 86400, "-noout")
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(self.cert, self.key)
        self.assertEqual(self.key.stat().st_mode & 0o777, 0o600)

    def test_valid_certificate_is_reused(self):
        self.assert_generated()
        before = (self.cert.read_bytes(), self.key.read_bytes())
        os.chmod(self.key, 0o644)
        self.assert_generated()
        self.assertEqual(before, (self.cert.read_bytes(), self.key.read_bytes()))
        self.assertEqual(self.key.stat().st_mode & 0o777, 0o600)

    def test_default_hostnames_are_in_certificate(self):
        # 未配置时三个默认入口均须通过证书地址校验。
        self.assert_generated()
        for host in DEFAULT_HOSTS:
            self.openssl("verify", "-CAfile", self.cert, "-verify_hostname", host, self.cert)

    def test_external_hostname_change_renews_certificate(self):
        self.assert_generated()
        before = self.cert.read_bytes()
        result = self.generate(external_hosts="other-external.test")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotEqual(before, self.cert.read_bytes())
        self.openssl("verify", "-CAfile", self.cert, "-verify_hostname", "other-external.test", self.cert)

    def test_empty_domain_list_uses_default(self):
        # 空字符串与未设置列表一致，仍生成覆盖三个默认域名的证书。
        result = self.generate(external_hosts="")
        self.assertEqual(result.returncode, 0, result.stderr)
        for host in DEFAULT_HOSTS:
            self.openssl("verify", "-CAfile", self.cert, "-verify_hostname", host, self.cert)

    def test_all_domains_are_covered_without_duplicates(self):
        # 每个域名都需要通过实际证书地址校验，重复项不写入 SAN。
        domains = " first.lab.test, second.lab.test,first.lab.test "
        result = self.generate(external_hosts=domains)
        self.assertEqual(result.returncode, 0, result.stderr)
        for host in ("first.lab.test", "second.lab.test"):
            self.openssl("verify", "-CAfile", self.cert, "-verify_hostname", host, self.cert)
        details = self.openssl("x509", "-in", self.cert, "-text", "-noout")
        for host in DEFAULT_HOSTS:
            self.assertNotIn("DNS:" + host, details)
        self.assertEqual(details.count("DNS:first.lab.test"), 1)
        before = (self.cert.read_bytes(), self.key.read_bytes())
        result = self.generate(external_hosts=domains)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(before, (self.cert.read_bytes(), self.key.read_bytes()))

    def test_adding_domain_renews_certificate(self):
        self.assert_generated()
        before = self.cert.read_bytes()
        result = self.generate(external_hosts="www.pz-target-web.com,second.lab.test")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotEqual(before, self.cert.read_bytes())
        for host in ("www.pz-target-web.com", "second.lab.test"):
            self.openssl("verify", "-CAfile", self.cert, "-verify_hostname", host, self.cert)

    def test_invalid_domain_list_preserves_certificate(self):
        # 校验失败发生在写入前，不留下半份新证书，也不回退到默认域名。
        self.assert_generated()
        before = (self.cert.read_bytes(), self.key.read_bytes())
        for domains in (" ", "first.lab.test,", "first.lab.test,,second.lab.test",
                        "first.lab.test,https://second.lab.test", "first.lab.test,*.lab.test",
                        "first.lab.test\nsecond.lab.test", "first.lab.test,DNS:bad.test"):
            with self.subTest(domains=domains):
                result = self.generate(external_hosts=domains)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(before, (self.cert.read_bytes(), self.key.read_bytes()))

    def test_changed_ip_replaces_certificate(self):
        self.assert_generated()
        before = self.cert.read_bytes()
        self.assert_generated("10.20.30.40")
        self.assertNotEqual(before, self.cert.read_bytes())
        self.openssl("verify", "-CAfile", self.cert, "-verify_ip",
                     "10.20.30.40", self.cert)

    def test_force_renews_certificate(self):
        self.assert_generated()
        before = self.cert.read_bytes()
        self.assert_generated(force=True)
        self.assertNotEqual(before, self.cert.read_bytes())

    def test_mismatched_key_is_repaired(self):
        self.assert_generated()
        self.openssl("genpkey", "-algorithm", "EC", "-pkeyopt",
                     "ec_paramgen_curve:prime256v1", "-out", self.key)
        self.assert_generated()
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(self.cert, self.key)

    def test_near_expiry_certificate_is_renewed(self):
        self.directory.mkdir()
        self.openssl(
            "req", "-x509", "-newkey", "ec", "-pkeyopt",
            "ec_paramgen_curve:prime256v1", "-nodes", "-days", "1",
            "-subj", "/CN=192.168.5.10", "-addext",
            "subjectAltName=IP:192.168.5.10,DNS:localhost,IP:127.0.0.1",
            "-keyout", self.key, "-out", self.cert,
        )
        before = self.cert.read_bytes()
        self.assert_generated()
        self.assertNotEqual(before, self.cert.read_bytes())
        self.openssl("x509", "-in", self.cert, "-checkend", 30 * 86400, "-noout")

    def test_corrupt_certificate_is_repaired(self):
        self.assert_generated()
        self.cert.write_text("invalid certificate\n")
        self.assert_generated()
        self.openssl("verify", "-CAfile", self.cert, self.cert)

    def test_invalid_ip_does_not_overwrite_existing_files(self):
        self.assert_generated()
        before = (self.cert.read_bytes(), self.key.read_bytes())
        for host in ("", "localhost", "999.1.1.1", "127.0.0.01", "1.2.3",
                     "192.168.5.10/invalid", "1.2.3.4\nDNS:bad", "::1"):
            with self.subTest(host=host):
                self.assertNotEqual(self.generate(host).returncode, 0)
                self.assertEqual(before, (self.cert.read_bytes(), self.key.read_bytes()))


if __name__ == "__main__":
    unittest.main()
