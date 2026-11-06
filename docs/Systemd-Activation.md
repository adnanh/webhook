# systemd socket 激活

Linux 上可由 systemd 创建监听 socket，再把文件描述符交给 webhook。这样进程可直接以普通用户运行，并让 systemd 管理监听端口。无需同时设置 `-socket`。

`/etc/systemd/system/webhook.socket`：

```ini
[Unit]
Description=Webhook listening socket

[Socket]
ListenStream=9000
NoDelay=true

[Install]
WantedBy=sockets.target
```

`/etc/systemd/system/webhook.service`：

```ini
[Unit]
Description=Webhook service
Requires=webhook.socket
After=network.target

[Service]
User=webhook
Group=webhook
WorkingDirectory=/var/lib/webhook
ExecStart=/usr/local/bin/webhook -c /etc/webhook/webhook.yaml
Restart=on-failure
```

先准备服务用户及目录，然后启用 socket：

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now webhook.socket
curl http://127.0.0.1:9000/status
systemctl status webhook.socket webhook.service
```

应用会检查 systemd 提供的激活监听器。此模式由 `ListenStream` 决定地址和端口，不能期待应用的 `-ip`/`-port` 改变该监听器。若用 `ListenStream=127.0.0.1:9000`，只接受回环请求；若需低端口也可让 systemd 持有它。

普通进程部署见[运维指南](Operations.md)。完整单元语义参见 [systemd.socket](https://www.freedesktop.org/software/systemd/man/latest/systemd.socket.html)。
