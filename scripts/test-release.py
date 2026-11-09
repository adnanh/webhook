#!/usr/bin/env python3
"""Test embedded release commands offline; never call the real GitHub CLI."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parent.parent
WORKFLOW = ROOT / ".github/workflows/release.yml"


def step_script(name):
    lines = WORKFLOW.read_text().splitlines()
    start = lines.index("      - name: " + name) + 1
    while lines[start] != "        run: |":
        start += 1
    end = start + 1
    while end < len(lines) and (not lines[end].strip() or lines[end].startswith("          ")):
        end += 1
    return textwrap.dedent("\n".join(lines[start + 1:end]))


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        (ROOT / "build").mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(prefix="release-tests-", dir=ROOT / "build")
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)
        self.output = self.work / "output"
        self.calls = self.work / "calls.jsonl"
        tools = self.work / "tools"
        tools.mkdir()
        stub = tools / "gh.py"
        stub.write_text('''import json, os, sys
from pathlib import Path
args = sys.argv[1:]
with open(os.environ["RELEASE_TEST_CALLS"], "a") as output:
    output.write(json.dumps(args) + "\\n")
if args[:2] == ["release", "view"]:
    state = os.environ.get("RELEASE_TEST_STATE", "missing")
    if state == "missing":
        sys.exit(1)
    print("true" if state == "draft" else "false")
if args[:2] == ["release", "upload"] and os.environ.get("RELEASE_TEST_UPLOAD_FAIL"):
    sys.exit(1)
if args[:2] == ["release", "upload"]:
    for name in args[3:]:
        if not name.startswith("--") and Path(name).stat().st_size == 0:
            print("HTTP 400: Bad Content-Length", file=sys.stderr)
            sys.exit(1)
''')
        gh = tools / "gh"
        gh.write_text('#!/bin/sh\nexec "$RELEASE_TEST_PYTHON" "$RELEASE_TEST_STUB" "$@"\n')
        gh.chmod(0o755)
        (self.work / "dist").mkdir()
        (self.work / "dist/checksums.txt").write_text("fixture")
        (self.work / "dist/webhook-linux-amd64.tar.gz").write_bytes(b"fixture")
        self.env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"],
                        TAG="v1.0.0", PRERELEASE="false", GITHUB_OUTPUT=str(self.output),
                        RELEASE_TEST_CALLS=str(self.calls), RELEASE_TEST_STUB=str(stub),
                        RELEASE_TEST_PYTHON=sys.executable)

    def run_step(self, name, **env):
        return subprocess.run(["bash", "-e", "-o", "pipefail", "-c", step_script(name)],
                              cwd=self.work, env=dict(self.env, **env), capture_output=True, text=True, timeout=30)

    def gh_calls(self):
        return [json.loads(line) for line in self.calls.read_text().splitlines()]

    def assert_success(self, result):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_version_normalization(self):
        for value, tag, prerelease in [("1.0.0", "v1.0.0", "false"), ("v1.0.0", "v1.0.0", "false"),
                                       ("v1.0.0-rc.1", "v1.0.0-rc.1", "true"),
                                       ("v1.0.0+build-1", "v1.0.0+build-1", "false")]:
            with self.subTest(value=value):
                self.output.write_text("")
                self.assert_success(self.run_step("Validate and normalize version", REQUESTED_VERSION=value))
                values = dict(line.split("=", 1) for line in self.output.read_text().splitlines())
                self.assertEqual(values, dict(tag=tag, version=tag[1:], prerelease=prerelease))

    def test_unsigned_release_uploads_nonempty_placeholder_and_matching_checksum(self):
        dist = self.work / "dist"
        for name in ("webhook-windows-amd64.zip", "install.sh", "install.ps1", "install.cmd", "update-manifest.json"):
            (dist / name).write_text("fixture")
        self.assert_success(self.run_step("Sign update manifest", UPDATE_SIGNING_PRIVATE_KEY="", UPDATE_MANIFEST_PUBLIC_KEY=""))
        signature = (dist / "update-manifest.json.sig").read_bytes()
        self.assertTrue(signature, "GitHub rejects zero-byte release assets")
        self.assertEqual(signature.strip(), b"", "Unsigned placeholder must not contain a signature")
        checksum = hashlib.sha256(signature).hexdigest()
        self.assertIn(f"{checksum}  update-manifest.json.sig", (dist / "checksums.txt").read_text())
        self.assert_success(self.run_step("Publish complete GitHub release"))
        self.assertEqual(self.gh_calls()[-1][1], "edit")

    def test_invalid_versions_rejected(self):
        for value in ("latest", "v1.0", "v1.0.0\nversion=bad", "$(exit 0)", "v1.0.0;exit 0"):
            with self.subTest(value=value):
                result = self.run_step("Validate and normalize version", REQUESTED_VERSION=value)
                self.assertNotEqual(result.returncode, 0)

    def test_stable_release_explicitly_becomes_latest(self):
        self.assert_success(self.run_step("Publish complete GitHub release"))
        calls = self.gh_calls()
        self.assertEqual([call[1] for call in calls], ["view", "create", "upload", "edit"])
        self.assertIn("--draft", calls[1])
        self.assertIn("--latest", calls[-1])
        self.assertIn("--prerelease=false", calls[-1])
        self.assertIn("dist/checksums.txt", calls[2])
        self.assertIn("dist/webhook-linux-amd64.tar.gz", calls[2])

    def test_prerelease_does_not_become_latest(self):
        self.assert_success(self.run_step("Publish complete GitHub release", TAG="v1.1.0-rc.1", PRERELEASE="true"))
        edit = self.gh_calls()[-1]
        self.assertIn("--prerelease", edit)
        self.assertIn("--latest=false", edit)
        self.assertNotIn("--latest", edit)

    def test_resume_draft(self):
        self.assert_success(self.run_step("Publish complete GitHub release", RELEASE_TEST_STATE="draft"))
        self.assertEqual([call[1] for call in self.gh_calls()], ["view", "upload", "edit"])

    def test_published_release_cannot_be_overwritten(self):
        result = self.run_step("Publish complete GitHub release", RELEASE_TEST_STATE="published")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual([call[1] for call in self.gh_calls()], ["view"])

    def test_failed_upload_keeps_release_unpublished(self):
        result = self.run_step("Publish complete GitHub release", RELEASE_TEST_UPLOAD_FAIL="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual([call[1] for call in self.gh_calls()], ["view", "create", "upload"])

    def test_empty_asset_is_rejected_before_creating_a_draft(self):
        (self.work / "dist/update-manifest.json.sig").write_bytes(b"")
        result = self.run_step("Publish complete GitHub release")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Release asset is empty", result.stdout)
        self.assertFalse(self.calls.exists())


if __name__ == "__main__":
    unittest.main(verbosity=2)
