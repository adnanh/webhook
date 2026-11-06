#!/usr/bin/env python3
"""Build and verify the documented demos locally using only Python's stdlib."""
import base64
import concurrent.futures
import contextlib
import hashlib
import hmac
import http.client
import json
import os
from pathlib import Path
import secrets
import shutil
import socket
import struct
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
EXAMPLES = ROOT / "examples"
SUFFIX = ".exe" if os.name == "nt" else ""
BINARY = ROOT / "build" / ("webhook" + SUFFIX)
DEMO = ROOT / "build" / ("demo-hook" + SUFFIX)
ENV = dict(os.environ, DEMO_HMAC_SECRET=secrets.token_hex(32), DEMO_NODE_TOKEN=secrets.token_hex(32))
# Local smoke traffic must not be sent through a machine's configured proxy.
HTTP = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def check(condition, message):
    if not condition:
        raise RuntimeError(message)


def request(port, path, method="POST", body=None, headers=None):
    req = urllib.request.Request(f"http://127.0.0.1:{port}{path}", data=body, headers=headers or {}, method=method)
    try:
        response = HTTP.open(req, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        return response.code, response.read().decode("utf-8"), response.headers


def eventually(action, predicate, description):
    deadline = time.monotonic() + 8
    last = None
    while time.monotonic() < deadline:
        try:
            last = action()
            if predicate(last):
                return last
        except (OSError, urllib.error.URLError, http.client.HTTPException) as error:
            last = str(error)
        time.sleep(0.1)
    raise RuntimeError(f"等待{description}失败: {last}")


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


@contextlib.contextmanager
def server(temp, label, args, port=None):
    if port is None:
        port = free_port()
    log = temp / (label + ".log")
    with log.open("wb") as output:
        process = subprocess.Popen([str(BINARY), *args, "-ip", "127.0.0.1", "-port", str(port)], cwd=ROOT, env=ENV, stdout=output, stderr=output)
        try:
            eventually(lambda: request(port, "/", "GET"), lambda r: r[0] == 200, label + "启动")
            yield port
        except Exception as error:
            raise RuntimeError(f"{label}: {error}\n{log.read_text(encoding='utf-8', errors='replace')}") from error
        finally:
            process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()


def copy_hooks(temp, relative):
    target = temp / relative.replace("/", "-")
    # Only the executable path changes for native Windows.
    target.write_text((EXAMPLES / relative).read_text(encoding="utf-8").replace("./build/demo-hook", DEMO.as_posix()), encoding="utf-8")
    return target


def run(temp):
    basic = copy_hooks(temp, "basic/hooks.yaml")
    basic_config = temp / "basic.yaml"
    basic_config.write_text((EXAMPLES / "basic/webhook.yaml").read_text(encoding="utf-8").replace("examples/basic/hooks.yaml", basic.as_posix()), encoding="utf-8")
    headers = {"Content-Type": "application/json", "X-Demo-Token": "local-demo-only", "X-Trace": "smoke"}
    payload = json.dumps({"user": {"name": "演示"}}, ensure_ascii=False).encode()
    with server(temp, "basic", ["-c", str(basic_config)]) as port:
        code, body, _ = request(port, "/hooks/hello?env=dev", body=payload, headers=headers)
        check(code == 200 and json.loads(body) == {"arguments": ["演示", "dev", "POST"], "trace": "smoke"}, "参数/环境映射失败")
        check(request(port, "/hooks/hello?env=dev")[0] == 403, "缺少 token 未拒绝")
        check(request(port, "/hooks/hello?env=dev", "GET")[0] == 405, "GET 未拒绝")
        status = json.loads(request(port, "/status", "GET")[1])
        check(status["status"] == "ok" and status["hooks"]["count"] == 1, "状态不正确")
    print("通过：参数、环境、组合规则与状态")

    signed = copy_hooks(temp, "signature/hooks.yaml.tmpl")
    with server(temp, "signature", ["-hooks", str(signed), "-template"]) as port:
        body = (EXAMPLES / "signature/payload.json").read_bytes()
        signature = hmac.new(ENV["DEMO_HMAC_SECRET"].encode(), body, hashlib.sha256).hexdigest()
        headers_signed = {"Content-Type": "application/json", "X-Hub-Signature-256": "sha256=" + signature}
        check(request(port, "/hooks/signed", body=body, headers=headers_signed)[0] == 200, "合法签名失败")
        check(request(port, "/hooks/signed", body=b'{}', headers=headers_signed)[0] == 500, "篡改请求未拒绝")
    print("通过：模板与 HMAC 签名")

    files = copy_hooks(temp, "files/hooks.yaml")
    with server(temp, "files", ["-hooks", str(files)]) as port:
        boundary = secrets.token_hex(16)
        file_body = (f"--{boundary}\r\nContent-Disposition: form-data; name=\"pkg\"; filename=\"hello.txt\"\r\nContent-Type: text/plain\r\n\r\n你好，webhook！\n\r\n--{boundary}--\r\n").encode()
        code, body, _ = request(port, "/hooks/upload", body=file_body, headers={"Content-Type": "multipart/form-data; boundary=" + boundary})
        check(code == 200 and body == "你好，webhook！\n", "上传文件失败")
        code, body, _ = request(port, "/hooks/base64", body=b'{"content":"aGVsbG8K"}', headers={"Content-Type": "application/json"})
        check(code == 200 and body == "hello\n", "Base64 文件失败")
        code, body, _ = request(port, "/hooks/nested-json", body=b'metadata=%7B%22version%22%3A%221.2.3%22%7D', headers={"Content-Type": "application/x-www-form-urlencoded"})
        check(code == 200 and json.loads(body)["arguments"] == ["1.2.3"], "表单 JSON 失败")
    print("通过：上传、Base64 参数文件与表单 JSON")

    execution = copy_hooks(temp, "execution/hooks.yaml")
    with server(temp, "execution", ["-hooks", str(execution)]) as port:
        check(request(port, "/hooks/timeout")[0] == 500, "超时未失败")
        check(request(port, "/hooks/async")[:2] == (202, "accepted"), "异步响应失败")
        code, body, _ = request(port, "/hooks/failure")
        check(code == 500 and "demo failure" in body, "失败输出不正确")
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
            results = list(pool.map(lambda _: request(port, "/hooks/limited")[0], range(2)))
        check(sorted(results) == [200, 503], "并发限制不正确")
    print("通过：超时、并发、异步与失败输出")

    hooks_dir = temp / "hooks"
    hooks_dir.mkdir()
    ping = copy_hooks(temp, "hotreload/hooks/ping.yaml")
    shutil.copyfile(ping, hooks_dir / "ping.yaml")
    hot_config = temp / "hotreload.yaml"
    hot_config.write_text((EXAMPLES / "hotreload/webhook.yaml").read_text(encoding="utf-8").replace("examples/hotreload/hooks", hooks_dir.as_posix()), encoding="utf-8")
    with server(temp, "hotreload", ["-c", str(hot_config)]) as port:
        check(request(port, "/hooks/ping")[0] == 200, "初始目录加载失败")
        pong = hooks_dir / "pong.yaml"
        pong.write_text(ping.read_text(encoding="utf-8").replace("id: ping", "id: pong"), encoding="utf-8")
        eventually(lambda: request(port, "/hooks/pong"), lambda r: r[0] == 200, "新增 hook")
        pong.unlink()
        eventually(lambda: request(port, "/hooks/pong"), lambda r: r[0] == 404, "删除 hook")
    print("通过：目录自动发现与热加载")

    secret = secrets.token_bytes(20)
    admin_config = temp / "admin.yaml"
    text = (EXAMPLES / "admin/webhook.yaml.example").read_text(encoding="utf-8")
    text = text.replace("REPLACE_WITH_RANDOM_BASE32_SECRET", base64.b32encode(secret).decode()).replace("REPLACE_WITH_INDEPENDENT_RANDOM_SECRET", secrets.token_hex(32)).replace("build/admin-hooks.yaml", basic.as_posix())
    admin_config.write_text(text, encoding="utf-8")
    with server(temp, "admin", ["-c", str(admin_config)]) as port:
        check(request(port, "/admin/api/config", "GET")[0] == 401, "未认证管理 API 未拒绝")
        digest = hmac.new(secret, struct.pack(">Q", int(time.time()) // 30), hashlib.sha1).digest()
        offset = digest[-1] & 15
        code = (struct.unpack(">I", digest[offset:offset+4])[0] & 0x7fffffff) % 1000000
        status, _, response_headers = request(port, "/admin/api/auth/login", body=json.dumps({"code": f"{code:06}"}).encode(), headers={"Content-Type": "application/json"})
        check(status == 200, "TOTP 登录失败")
        cookie = response_headers["Set-Cookie"].split(";", 1)[0]
        status, body, _ = request(port, "/admin/api/config", "GET", headers={"Cookie": cookie})
        check(status == 200 and len(json.loads(body)["files"]) == 1, "认证配置查询失败")
        mutation = {"file": basic.as_posix(), "hook": {"id": "admin-demo", "execute-command": DEMO.as_posix(), "http-methods": ["POST"], "include-command-output-in-response": True, "pass-arguments-to-command": [{"source": "string", "name": "echo"}]}}
        check(request(port, "/admin/api/hooks", body=json.dumps(mutation).encode(), headers={"Cookie": cookie, "Content-Type": "application/json"})[0] == 201, "在线新增失败")
        check(request(port, "/hooks/admin-demo")[0] == 200, "在线新增未生效")
    print("通过：Admin TOTP、会话与在线编辑")

    hub_port = free_port()
    agent_config = temp / "agent.yaml"
    text = (EXAMPLES / "network/agent.yaml").read_text(encoding="utf-8").replace("examples/basic/hooks.yaml", basic.as_posix()).replace("127.0.0.1:19000", f"127.0.0.1:{hub_port}")
    agent_config.write_text(text, encoding="utf-8")
    with server(temp, "hub", ["-c", str(EXAMPLES / "network/hub.yaml")], hub_port):
        with (temp / "agent.log").open("wb") as output:
            agent = subprocess.Popen([str(BINARY), "-c", str(agent_config)], cwd=ROOT, env=ENV, stdout=output, stderr=output)
            try:
                response = eventually(lambda: request(hub_port, "/demo/hello?env=dev", body=payload, headers=headers), lambda r: r[0] == 200, "Agent 上线与 Hub 路由")
                check(json.loads(response[1])["arguments"] == ["演示", "dev", "POST"], "Hub 转发内容不正确")
            finally:
                agent.terminate()
                try:
                    agent.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    agent.kill()
                    agent.wait()
    print("通过：Hub/Agent 进程内转发")


def main():
    (ROOT / "build").mkdir(exist_ok=True)
    for output, target in [(BINARY, "./cmd/webhook"), (DEMO, "./examples/demo-hook")]:
        subprocess.run(["go", "build", "-o", str(output), target], cwd=ROOT, check=True)
    with tempfile.TemporaryDirectory(prefix="demo-smoke-", dir=ROOT / "build") as temp:
        run(Path(temp))
    print("全部演示验证通过，临时配置、随机密钥与进程已清理。")


if __name__ == "__main__":
    main()
