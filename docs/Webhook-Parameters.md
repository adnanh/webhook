# Webhook parameters
```
Usage of webhook:
  -access-blacklist string
        comma-separated list of client IPs or CIDRs denied from accessing the service
  -access-whitelist string
        comma-separated list of client IPs or CIDRs allowed to access the service
  -cert string
        path to the HTTPS certificate pem file (default "cert.pem")
  -cipher-suites string
        comma-separated list of supported TLS cipher suites
  -command-timeout string
        default timeout for hook command execution (for example 30s); 0 disables the timeout
  -debug
        show debug output
  -header value
        response header to return, specified in format name=value, use multiple times to set multiple headers
  -hooks value
        path to the json file containing defined hooks the webhook should serve, use multiple times to load from different files
  -hooks-dir value
        directory containing JSON or YAML hooks files; use multiple times to load and watch multiple directories
  -hotreload
        watch hooks file for changes and reload them automatically
  -http-methods string
        set default allowed HTTP methods (ie. "POST"); separate methods with comma
  -ip string
        ip the webhook should serve hooks on (default "0.0.0.0")
  -key string
        path to the HTTPS certificate private key pem file (default "key.pem")
  -list-cipher-suites
        list available TLS cipher suites
  -logfile string
        send log output to a file; implicitly enables verbose logging
  -max-concurrency int
        default maximum number of concurrent executions per hook; 0 disables the limit
  -max-body-size int
        maximum webhook request body size in bytes (0 disables the limit) (default 10485760)
  -max-multipart-mem int
        maximum memory in bytes for parsing multipart form data before disk caching (default 1048576)
  -nopanic
        do not panic if hooks cannot be loaded when webhook is not running in verbose mode
  -pidfile string
        create PID file at the given path
  -port int
        port the webhook should serve hooks on (default 9000)
  -secure
        use HTTPS instead of HTTP
  -setgid int
        set group ID after opening listening port; must be used with setuid
  -setuid int
        set user ID after opening listening port; must be used with setgid
  -socket string
        path to a Unix socket (e.g. /tmp/webhook.sock) or Windows named pipe (e.g. \\.\pipe\webhook) to use instead of listening on an ip and port; if specified, the ip and port options are ignored
  -status-path string
        URL path for the service status endpoint (default "status")
  -template
        parse hooks file as a Go template
  -tls-min-version string
        minimum TLS version (1.0, 1.1, 1.2, 1.3) (default "1.2")
  -urlprefix string
        url prefix to use for served hooks (protocol://yourserver:port/PREFIX/:hook-id) (default "hooks")
  -verbose
        show verbose output
  -version
        display webhook version and quit
  -update-enabled
        enable update checks in the admin API (default true)
  -update-repository string
        GitHub repository used for updates (default "xtulnx/webhook")
  -update-state-dir string
        directory for update state; defaults to the current working directory
  -x-request-id
        use X-Request-Id header, if present, as request ID
  -x-request-id-limit int
        truncate X-Request-Id header to limit; default no limit
```

Use any of the above specified flags to override their default behavior.

`-command-timeout` and `-max-concurrency` act as defaults for all hooks. Individual hooks can override them using the `command-timeout` and `max-concurrency` hook properties. Within a hook definition, `0` explicitly disables the inherited limit.

## Access control and request limits

`-access-whitelist` and `-access-blacklist` apply to every HTTP endpoint, including hooks, status, and the admin API. Values may be IPv4/IPv6 addresses or CIDR ranges, separated by commas. Configure only one; when a whitelist is configured, all other addresses are denied. IP matching uses the resolved client address, and only a request from a `-trusted-proxies` address may supply `-real-ip-header`.

`-max-body-size` limits non-multipart and multipart request bodies before parsing. It defaults to 10 MiB; set it to `0` only when an unlimited body is explicitly required.

# Live reloading hooks
If you are running an OS that supports the HUP or USR1 signal, you can use it to trigger hooks reload from hooks file, without restarting the webhook instance.
```bash
kill -USR1 webhookpid

kill -HUP webhookpid
```

`-hooks-dir` loads all direct child files ending in `.json`, `.yaml`, or `.yml` and automatically watches the directory, even when `-hotreload` is not set. New and removed files are reflected at runtime. A malformed update or a duplicate hook ID is rejected while the last valid configuration remains active. Other file types, subdirectories, and symbolic links that resolve outside the configured directory are ignored. Existing `-hooks` files continue to require `-hotreload` for automatic reloads.

# Service status

`GET /status` returns service uptime, the current hook and source-file counts, hook IDs, the most recent successful load time, and watcher state. Use `-status-path` to move the endpoint. The response intentionally omits absolute paths, command definitions, environment mappings, and load error details.
