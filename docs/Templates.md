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

## Changing the template delimiters

If your hook configuration includes lookup arguments of type `{"source": "template"}`, and you also need to parse the hooks file _as_ a template, you can use the `-template-delims` parameter to change the template delimiter used when processing the hook file so it does not clash with the standard `{{ ... }}` delimiters used for template lookups.  The parameter is a comma-separated pair of the left and right delimiter strings, e.g. `-template-delims='[[,]]'` would use square brackets.  For a configuration like this:

```json
[
  {
    "id": "example",
    "trigger-rule": {
      "check-signature": {
        "algorithm": "sha256",
        "secret": "[[ getenv `XXXTEST_SECRET` | js ]]",
        "signature": {
          "source": "header",
          "name": "X-Signature"
        },
        "string-to-sign": {
          "source": "template",
          "name": "{{ .BodyText }}{{ .GetHeader `date` }}"
        }
      }
    }
  }
]
```

the `-template-delims='[[,]]'` setting would cause the `getenv` part to be interpreted when parsing the hook file, whereas the string-to-sign template would be executed when evaluating the trigger rule against each request.

[w]: https://github.com/adnanh/webhook
[tt]: https://golang.org/pkg/text/template/
