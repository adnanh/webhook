# Templates in Webhook

[`webhook`][w] can parse a hooks configuration file as a Go template when given the `-template` [CLI parameter](Webhook-Parameters.md).

In additional to the [built-in Go template functions and features][tt], `webhook` provides a `getenv` template function for inserting environment variables into a templated configuration file.

## Example Usage

In the example JSON template file below (YAML is also supported), the `payload-hmac-sha1` matching rule looks up the HMAC secret from the environment using the `getenv` template function.
Additionally, the result is piped through the built-in Go template function `js` to ensure that the result is a well-formed Javascript/JSON string.

```json
[
  {
    "id": "webhook",
    "execute-command": "/home/adnan/redeploy-go-webhook.sh",
    "command-working-directory": "/home/adnan/go",
    "response-message": "I got the payload!",
    "response-headers": [
      {
        "name": "Access-Control-Allow-Origin",
        "value": "*"
      }
    ],
    "pass-arguments-to-command": [
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
    "trigger-rule": {
      "and": [
        {
          "match": {
            "type": "payload-hmac-sha1",
            "secret": "{{ getenv "XXXTEST_SECRET" | js }}",
            "parameter": {
              "source": "header",
              "name": "X-Hub-Signature"
            }
          }
        },
        {
          "match": {
            "type": "value",
            "value": "refs/heads/master",
            "parameter": {
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

[w]: https://github.com/adnanh/webhook
[tt]: https://golang.org/pkg/text/template/
