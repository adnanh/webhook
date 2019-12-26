# Webhook parameters
```
Usage of webhook:
  -cert string
        path to the HTTPS certificate pem file (default "cert.pem")
  -cipher-suites string
        comma-separated list of supported TLS cipher suites
  -debug
        show debug output
  -header value
        response header to return, specified in format name=value, use multiple times to set multiple headers
  -hooks value
        path to the json file containing defined hooks the webhook should serve, use multiple times to load from different files
  -hotreload
        watch hooks file for changes and reload them automatically
  -ip string
        ip the webhook should serve hooks on (default "0.0.0.0")
  -key string
        path to the HTTPS certificate private key pem file (default "key.pem")
  -list-cipher-suites
        list available TLS cipher suites
  -nopanic
        do not panic if hooks cannot be loaded when webhook is not running in verbose mode
  -port int
        port the webhook should serve hooks on (default 9000)
  -secure
        use HTTPS instead of HTTP
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
  -x-request-id
        use X-Request-Id header, if present, as request ID
  -x-request-id-limit int
        truncate X-Request-Id header to limit; default no limit
```

Use any of the above specified flags to override their default behavior.

# Live reloading hooks
If you are running an OS that supports the HUP or USR1 signal, you can use it to trigger hooks reload from hooks file, without restarting the webhook instance.
```bash
kill -USR1 webhookpid

kill -HUP webhookpid
```
