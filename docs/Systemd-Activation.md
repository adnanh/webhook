# Using systemd socket activation

_New in v2.9.0_

On platforms that use [systemd](https://systemd.io), [webhook][w] 
supports the _socket activation_ mechanism.  In this mode, systemd itself is responsible for managing the listening socket, and it launches [webhook][w] the first time it receives a request on the socket.  This has a number of advantages over the standard mode:

- [webhook][w] can run as a normal user while still being able to use a port number like 80 or 443 that would normally require root privilege
- if the [webhook][w] process dies and is restarted, pending connections are not dropped - they just keep waiting until the restarted [webhook][w] is ready

No special configuration is necessary to tell [webhook][w] that socket activation is being used - socket activation sets specific environment variables when launching the activated service, if [webhook][w] detects these variables it will ignore the `-port` and `-socket` options and simply use the systemd-provided socket instead of opening its own.

## Configuration
To run [webhook][w] with socket activation you need to create _two_ separate unit files in your systemd configuration directory (typically `/etc/systemd/system`), one for the socket and one for the service.  They must have matching names; in this example we use `webhook.socket` and `webhook.service`.  At their simplest, these files should look like:

**webhook.socket**
```
[Unit]
Description=Webhook server socket

[Socket]
# Listen on all network interfaces, port 9000
ListenStream=9000

# Alternatives:

## Listen on one specific interface only
# ListenStream=10.0.0.1:9000
# FreeBind=true

## Listen on a Unix domain socket
# ListenStream=/tmp/webhook.sock

[Install]
WantedBy=multi-user.target
```

**webhook.service**
```
[Unit]
Description=Webhook server

[Service]
Type=exec
ExecStart=webhook -nopanic -hooks /etc/webhook/hooks.yml

# Which user should the webhooks run as?
User=nobody
Group=nogroup
```

You should enable and start the _socket_, but it is not necessary to enable the _service_ - this will be started automatically when the socket receives its first request.

```sh
sudo systemctl enable webhook.socket
sudo systemctl start webhook.socket
```

Systemd unit files support many other options, see the [systemd.socket](https://www.freedesktop.org/software/systemd/man/latest/systemd.socket.html) and [systemd.service](https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html) manual pages for full details.

[w]: https://github.com/adnanh/webhook
