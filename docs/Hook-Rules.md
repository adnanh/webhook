# Hook rules

### Table of Contents

* [And](#and)
* [Or](#or)
* [Not](#not)
* [Multi-level](#multi-level)
* [Match](#match)
  * [Match value](#match-value)
  * [Match regex](#match-regex)
  * [Match Whitelisted IP range](#match-whitelisted-ip-range)
  * [Match scalr-signature](#match-scalr-signature)
* [Check signature](#check-signature)
  * [Match payload-hmac-sha1](#match-payload-hmac-sha1)
  * [Match payload-hmac-sha256](#match-payload-hmac-sha256)
  * [Match payload-hmac-sha512](#match-payload-hmac-sha512)

## And
*And rule* will evaluate to _true_, if and only if all of the sub rules evaluate to _true_.
```json
{
"and":
  [
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
    },
    {
      "match":
      {
        "type": "regex",
        "regex": ".*",
        "parameter":
        {
          "source": "payload",
          "name": "repository.owner.name"
        }
      }
    }
  ]
}
```
## Or
*Or rule* will evaluate to _true_, if any of the sub rules evaluate to _true_.
```json
{
"or":
  [
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
    },
    {
      "match":
      {
        "type": "value",
        "value": "refs/heads/development",
        "parameter":
        {
          "source": "payload",
          "name": "ref"
        }
      }
    }
  ]
}
```
## Not
*Not rule* will evaluate to _true_, if and only if the sub rule evaluates to _false_.
```json
{
"not":
  {
    "match":
    {
      "type": "value",
      "value": "refs/heads/development",
      "parameter":
      {
        "source": "payload",
        "name": "ref"
      }
    }
  }
}
```
## Multi-level
```json
{
    "and": [
    {
        "match": {
            "parameter": {
                "source": "header",
                "name": "X-Hub-Signature"
            },
            "type": "payload-hmac-sha1",
            "secret": "mysecret"
        }
    },
    {
        "or": [
        {
            "match":
            {
                "parameter":
                {
                    "source": "payload",
                    "name": "ref"
                },
                "type": "value",
                "value": "refs/heads/master"
            }
        },
        {
            "match":
            {
                "parameter":
                {
                    "source": "header",
                    "name": "X-GitHub-Event"
                },
                "type": "value",
                "value": "ping"
            }
        }
        ]
    }
    ]
}
```
## Match
*Match rule* will evaluate to _true_, if and only if the referenced value in the `parameter` field satisfies the `type`-specific rule.

*Please note:* Due to technical reasons, _number_ and _boolean_ values in the _match rule_ must be wrapped around with a pair of quotes.

### Match value
```json
{
  "match":
  {
    "type": "value",
    "value": "refs/heads/development",
    "parameter":
    {
      "source": "payload",
      "name": "ref"
    }
  }
}
```

### Match regex
For the regex syntax, check out <http://golang.org/pkg/regexp/syntax/>
```json
{
  "match":
  {
    "type": "regex",
    "regex": ".*",
    "parameter":
    {
      "source": "payload",
      "name": "ref"
    }
  }
}
```

### Match Whitelisted IP range

The IP can be IPv4- or IPv6-formatted, using [CIDR notation](https://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing#CIDR_blocks).  To match a single IP address only, use `/32`.

```json
{
  "match":
  {
    "type": "ip-whitelist",
    "ip-range": "192.168.0.1/24"
  }
}
```

Note this does not work if webhook is running behind a reverse proxy, as the "client IP" will either not be available at all (if webhook is using a Unix socket or named pipe) or it will be the address of the _proxy_, not of the real client.  You will probably need to enforce client IP restrictions in the reverse proxy itself, before forwarding the requests to webhook.

### Match scalr-signature

The trigger rule checks the scalr signature and also checks that the request was signed less than 5 minutes before it was received. 
A unique signing key is generated for each webhook endpoint URL you register in Scalr.
Given the time check make sure that NTP is enabled on both your Scalr and webhook server to prevent any issues

```json
{
  "match":
  {
    "type": "scalr-signature",
    "secret": "Scalr-provided signing key"
  }
}
```

## Check Signature

Many webhook protocols involve the hook sender computing an [HMAC](https://en.wikipedia.org/wiki/HMAC) _signature_ over the request content using a shared secret key, and sending the expected signature value as part of the webhook call.  The webhook recipient can then compute their own value for the signature using the same secret key and verify that value against the one supplied by the sender.  Since the sender and receiver are (or at least _should be_) the only parties that have knowledge of the secret, a matching signature guarantees that the payload is valid and was created by the legitimate sender.

The `"check-signature"` rule type is used to validate these kinds of signatures.  In its simplest form you just specify the _algorithm_ (`sha1`, `sha256` or `sha512`), the _secret_, and where in the request to find the signature (typically a header or a query parameter).  Webhook will compute the HMAC over the whole of the request body using the supplied secret, and compare the result to the one taken from the request

```json
{
  "check-signature":
  {
    "algorithm": "sha256",
    "secret": "yoursecret",
    "signature":
    {
      "source": "header",
      "name": "X-Hub-Signature"
    }
  }
}
```

Note that if multiple signatures were passed via a comma separated string, each
will be tried unless a match is found, and any `algorithm=` prefix is stripped off
each signature value before comparison.  This allows for cases where the sender includes
several signatures with different algorithms in the same header, e.g.:

```
X-Hub-Signature: sha1=the-sha1-signature,sha256=the-sha256-signature
```

If the sender computes the signature over something other than just the request body then you can optionally provide a `"string-to-sign"` argument.  Usually this will be a template that assembles the string-to-sign from different parts of the request (one of which could be the body).  For example this would compute a signature over the values of the `X-Request-Id` header, `Date` header, and request body, separated by line breaks:

```yaml
check-signature:
  algorithm: sha512
  secret: 5uper5eecret
  signature:
    source: header
    name: X-Hook-Signature
  string-to-sign:
    source: template
    name: |
      {{- printf "%s\r\n" (.GetHeader "x-request-id") -}}
      {{- printf "%s\r\n" (.GetHeader "date") -}}
      {{- .BodyText -}}
```

Note that signature algorithms can be very particular about whether "line breaks" are unix style LF or Windows-style CR+LF.  It is safest to be explicit, as in the above example, using `{{- -}}` blocks (that ignore the white space within the template itself either side of the block) and `printf` with `\n` or `\r\n`, to ensure the template generates the correct style of line endings whatever platform you created it on.

### Legacy "match" rules for signatures
In previous versions of webhook signature verification was handled by a set of specific "match" rule types named `payload-hmac-<algorithm>` - the legacy format is still understood but you may wish to update your existing configurations to the new format.

The legacy configuration

```json
{
  "match":
  {
    "type": "payload-hmac-<type>",
    "secret": "secret",
    "parameter":
    {
      "source": "header",
      "name": "X-Signature"
    }
  }
}
```

is equivalent to the new style

```json
{
  "check-signature":
  {
    "algorithm": "<type>",
    "secret": "secret",
    "signature":
    {
      "source": "header",
      "name": "X-Signature"
    }
  }
}
```
