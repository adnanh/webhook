#!/usr/bin/env python3
"""Exercise installers offline against real archives and a small native executable."""
import hashlib
import io
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import tarfile
import tempfile
import unittest
import zipfile

ROOT = Path(__file__).resolve().parent.parent
WINDOWS = os.name == "nt"
VERSION = "9.8.7"
TAG = "v" + VERSION


class InstallerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        (ROOT / "build").mkdir(exist_ok=True)
        cls.suite = tempfile.TemporaryDirectory(prefix="installer-tests-", dir=ROOT / "build")
        cls.suite_dir = Path(cls.suite.name)
        source = cls.suite_dir / "main.go"
        source.write_text('''package main
import ("fmt"; "os"; "time")
func main() {
    if len(os.Args) > 1 && os.Args[1] == "--hold" { time.Sleep(time.Hour); return }
    fmt.Println("webhook version 9.8.7")
}
''')
        cls.binary = cls.suite_dir / ("webhook.exe" if WINDOWS else "webhook")
        subprocess.run(["go", "build", "-o", str(cls.binary), str(source)], check=True, cwd=ROOT)
        cls.goos = "windows" if WINDOWS else platform.system().lower()
        cls.arch = {"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64",
                    "arm64": "arm64", "i386": "386", "i686": "386"}[platform.machine().lower()]
        cls.bundle_name = f"webhook-{cls.goos}-{cls.arch}"
        cls.asset = cls.bundle_name + (".zip" if WINDOWS else ".tar.gz")

    @classmethod
    def tearDownClass(cls):
        cls.suite.cleanup()

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=self.suite_dir)
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)
        # Spaces exercise quoting in both installers and the batch entrypoint.
        self.install_dir = self.work / "install with spaces"
        self.install_dir.mkdir()
        self.target = self.install_dir / ("custom-webhook.exe" if WINDOWS else "custom-webhook")
        self.env = dict(os.environ, WEBHOOK_REPO="fixtures/webhook", WEBHOOK_VERSION="latest",
                        WEBHOOK_INSTALL_DIR=str(self.install_dir), WEBHOOK_BINARY_NAME=self.target.name)
        # PowerShell temp files must stay in the test workspace too.
        self.env.update(TMP=str(self.work), TEMP=str(self.work), TMPDIR=str(self.work))
        self.make_archive()
        self.set_checksums()
        if WINDOWS:
            self.prepare_powershell()
        else:
            self.prepare_shell()

    def make_archive(self, member=None):
        name = member or f"{self.bundle_name}/{self.binary.name}"
        archive = self.work / self.asset
        if WINDOWS:
            with zipfile.ZipFile(archive, "w") as output:
                output.write(self.binary, name)
        else:
            with tarfile.open(archive, "w:gz") as output:
                output.add(self.binary, arcname=name)

    def set_checksums(self, text=None):
        if text is None:
            digest = hashlib.sha256((self.work / self.asset).read_bytes()).hexdigest()
            text = f"{digest}  {self.asset}\n"
        (self.work / "checksums.txt").write_text(text)
        (self.work / "release.json").write_text(json.dumps({"tag_name": TAG}, indent=2))

    def prepare_shell(self):
        tools = self.work / "tools"
        tools.mkdir()
        # Mock only transport. URL matching proves latest is resolved to one fixed tag.
        curl = tools / "curl"
        curl.write_text('''#!/bin/sh
set -eu
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output) output="$2"; shift 2 ;;
    --retry|--connect-timeout|--max-time) shift 2 ;;
    --*) shift ;;
    *) url="$1"; shift ;;
  esac
done
case "$url" in
  https://api.github.com/repos/fixtures/webhook/releases/latest) file=release.json ;;
  https://github.com/fixtures/webhook/releases/download/v9.8.7/*) file="${url##*/}" ;;
  *) echo "unexpected download URL: $url" >&2; exit 1 ;;
esac
cp "$FIXTURES/$file" "$output"
''')
        curl.chmod(0o755)
        self.env.update(PATH=str(tools) + os.pathsep + os.environ["PATH"], FIXTURES=str(self.work))
        self.command = ["sh", str(ROOT / "scripts/install.sh")]

    def prepare_powershell(self):
        wrapper = self.work / "run.ps1"
        wrapper.write_text('''$ErrorActionPreference = "Stop"
function Invoke-WebRequest {
    param($Uri, $OutFile, $TimeoutSec, [switch] $UseBasicParsing)
    if ($Uri -eq "https://api.github.com/repos/fixtures/webhook/releases/latest") {
        $Name = "release.json"
    } elseif ($Uri.StartsWith("https://github.com/fixtures/webhook/releases/download/v9.8.7/")) {
        $Name = $Uri.Substring($Uri.LastIndexOf('/') + 1)
    } else { throw "Unexpected download URL: $Uri" }
    Copy-Item -LiteralPath (Join-Path $env:FIXTURES $Name) -Destination $OutFile
}
# Do not persist test installation directories in the runner's user PATH.
$PreviousPath = [Environment]::GetEnvironmentVariable("Path", "User")
try {
    & (Join-Path $env:INSTALLER_ROOT "scripts/install.ps1")
} finally {
    [Environment]::SetEnvironmentVariable("Path", $PreviousPath, "User")
}
''')
        self.env.update(FIXTURES=str(self.work), INSTALLER_ROOT=str(ROOT))
        self.command = ["powershell.exe", "-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", str(wrapper)]

    def run_installer(self):
        return subprocess.run(self.command, env=self.env, capture_output=True, text=True, timeout=60, cwd=ROOT)

    def assert_success(self, result):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        probe = subprocess.run([str(self.target), "-version"], capture_output=True, text=True, check=True)
        self.assertEqual(probe.stdout.strip(), "webhook version " + VERSION)
        self.assertFalse(list(self.install_dir.glob(".*.tmp")))

    def assert_preserves_existing(self):
        self.target.write_bytes(b"previous version")
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.target.read_bytes(), b"previous version")

    def test_latest_install_and_repeat_update(self):
        self.assert_success(self.run_installer())
        self.assert_success(self.run_installer())

    def test_unprefixed_version(self):
        self.env["WEBHOOK_VERSION"] = VERSION
        self.assert_success(self.run_installer())

    def test_compact_latest_metadata(self):
        (self.work / "release.json").write_text(json.dumps({"tag_name": TAG}))
        self.assert_success(self.run_installer())

    @unittest.skipUnless(WINDOWS, "Legacy Windows tar.gz release")
    def test_legacy_tar_release(self):
        (self.work / self.asset).unlink()
        archive = self.work / (self.bundle_name + ".tar.gz")
        with tarfile.open(archive, "w:gz") as output:
            output.add(self.binary, arcname=f"{self.bundle_name}/webhook.exe")
        digest = hashlib.sha256(archive.read_bytes()).hexdigest()
        self.set_checksums(f"{digest}  {archive.name}\n")
        self.assert_success(self.run_installer())

    def test_download_failure_preserves_existing(self):
        (self.work / self.asset).unlink()
        self.assert_preserves_existing()

    def test_checksum_mismatch_preserves_existing(self):
        self.set_checksums("0" * 64 + "  " + self.asset + "\n")
        self.assert_preserves_existing()

    def test_missing_checksum_preserves_existing(self):
        self.set_checksums("0" * 64 + "  other-asset.tar.gz\n")
        self.assert_preserves_existing()

    def test_duplicate_checksum_preserves_existing(self):
        checksums = (self.work / "checksums.txt").read_text()
        self.set_checksums(checksums * 2)
        self.assert_preserves_existing()

    def test_corrupt_archive_preserves_existing(self):
        (self.work / self.asset).write_bytes(b"not an archive")
        self.set_checksums()
        self.assert_preserves_existing()

    def test_missing_binary_preserves_existing(self):
        self.make_archive(member=f"{self.bundle_name}/wrong-binary")
        self.set_checksums()
        self.assert_preserves_existing()

    def test_version_mismatch_preserves_existing(self):
        # Keep the native executable valid but change the version it reports.
        binary_data = self.binary.read_bytes()
        changed = binary_data.replace(b"webhook version 9.8.7", b"webhook version 9.8.6")
        self.assertNotEqual(changed, binary_data)
        if WINDOWS:
            with zipfile.ZipFile(self.work / self.asset, "w") as output:
                output.writestr(f"{self.bundle_name}/webhook.exe", changed)
        else:
            with tarfile.open(self.work / self.asset, "w:gz") as output:
                info = tarfile.TarInfo(f"{self.bundle_name}/webhook")
                info.size, info.mode = len(changed), 0o755
                output.addfile(info, io.BytesIO(changed))
        self.set_checksums()
        self.assert_preserves_existing()

    def test_invalid_binary_name(self):
        self.env["WEBHOOK_BINARY_NAME"] = "../outside"
        self.assertNotEqual(self.run_installer().returncode, 0)
        self.assertFalse((self.work / "outside").exists())

    def test_destination_directory_is_rejected(self):
        self.target.mkdir()
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(list(self.target.iterdir()), [])

    @unittest.skipIf(WINDOWS, "Windows uses the built-in Get-FileHash")
    def test_missing_hash_tool_preserves_existing(self):
        tools = self.work / "no-hash-tools"
        tools.mkdir()
        for name in ("sh", "uname", "tar", "awk", "sed", "install", "tr"):
            (tools / name).symlink_to(shutil.which(name))
        self.env["PATH"] = str(tools)
        self.assert_preserves_existing()

    @unittest.skipUnless(WINDOWS, "Windows batch entrypoint")
    def test_batch_propagates_failure(self):
        result = subprocess.run(["cmd.exe", "/d", "/c", str(ROOT / "scripts/install.cmd"),
                                 "-BinaryName", "../outside"], env=self.env, capture_output=True, timeout=30)
        self.assertNotEqual(result.returncode, 0)

    @unittest.skipIf(WINDOWS, "Windows requires stopping running executables")
    def test_update_running_unix_binary(self):
        shutil.copy2(self.binary, self.target)
        process = subprocess.Popen([str(self.target), "--hold"])
        try:
            self.assert_success(self.run_installer())
            self.assertIsNone(process.poll())
        finally:
            process.terminate()
            process.wait(timeout=10)


if __name__ == "__main__":
    unittest.main(verbosity=2)
