# Templates in Webhook

[`webhook`][w] can parse a hooks configuration file as a Go template when given the `-template` [CLI parameter](Webhook-Parameters.md).

In additional to the [built-in Go template functions and features][tt], `webhook` provides a `getenv` template function for inserting environment variables into a templated configuration file.

## Example Usage

In the example JSON template file below (YAML is also supported), the `payload-hmac-sha1` matching rule looks up the HMAC secret from the environment using the `getenv` template function.
Additionally, the result is piped through the built-in Go template function `js` to ensure that the result is a well-formed Javascript/JSON string.

```
[
  {
    "id": "webhook",
    "execute-command": "/home/adnan/redeploy-go-webhook.sh",
    "command-working-directory": "/home/adnan/go",
    "response-message": "I got the payload!",
    "response-headers":
    [
      {
        "name": "Access-Control-Allow-Origin",
        "value": "*"
      }
    ],
    "pass-arguments-to-command":
    [
      {
        "source": "payload",
        "name": "head_commit.id"
      },
      {
        "source": "payload",
        "name": "pusher.name"
      },
      {
        "source": "payload",
        "name": "pusher.email"
      }
    ],
    "trigger-rule":
    {
      "and":
      [
        {
          "match":
          {
            "type": "payload-hmac-sha1",
            "secret": "{{ getenv "XXXTEST_SECRET" | js }}",
            "parameter":
            {
              "source": "header",
              "name": "X-Hub-Signature"
            }
          }
        },
        {
          "match":
          {
            "type": "value",
            "value": "refs/heads/master",
            "parameter":
            {
              "source": "payload",
              "name": "ref"
            }
          }
        }
      ]
    }
  }
]

```

## Template Functions

In addition to the [built-in Go template functions and features][tt], `webhook` provides the following functions:

### `getenv`

The `getenv` template function can be used for inserting environment variables into a templated configuration file.

Example: 
```
"Secret": "{{getenv TEST_secret | js}}"
```

### `cat`

The `cat` template function can be used to read a file from the local filesystem. This is useful for reading secrets from files. If the file doesn't exist, it returns an empty string.

Example:
```
"secret": "{{ cat "/run/secrets/my-secret" | js }}"
```

### `credential`

The `credential` template function provides a way to retrieve secrets using [systemd's LoadCredential mechanism](https://www.freedesktop.org/software/systemd/man/systemd.exec.html#Credentials). It reads the file specified by the given name from the directory specified in the `CREDENTIALS_DIRECTORY` environment variable.

If `CREDENTIALS_DIRECTORY` is not set, it will fall back to using `getenv` to read the secret from an environment variable of the given name.

Example:
```
"secret": "{{ credential "my-secret" | js }}"
```

[w]: https://github.com/adnanh/webhook
[tt]: https://golang.org/pkg/text/template/
