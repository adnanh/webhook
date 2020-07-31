# Hook rules

### Table of Contents

* [And](#and)
* [Or](#or)
* [Not](#not)
* [Multi-level](#multi-level)
* [Match](#match)
  * [Match value](#match-value)
  * [Match regex](#match-regex)
  * [Match payload-hash-sha1](#match-payload-hash-sha1)
  * [Match payload-hash-sha256](#match-payload-hash-sha256)
  * [Match payload-hash-sha512](#match-payload-hash-sha512)
  * [Match Whitelisted IP range](#match-whitelisted-ip-range)
  * [Match scalr-signature](#match-scalr-signature)

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
            "type": "payload-hash-sha1",
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

There are three different match rules:

### 1. Match value
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

### 2. Match regex
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

### 3. Match payload-hash-sha1
```json
{
  "match":
  {
    "type": "payload-hash-sha1",
    "secret": "yoursecret",
    "parameter":
    {
      "source": "header",
      "name": "X-Hub-Signature"
    }
  }
}
```

Note that if multiple signatures were passed via a comma separated string, each
will be tried unless a match is found. For example:

```
X-Hub-Signature: sha1=the-first-signature,sha1=the-second-signature
```

### 4. Match payload-hash-sha256
```json
{
  "match":
  {
    "type": "payload-hash-sha256",
    "secret": "yoursecret",
    "parameter":
    {
      "source": "header",
      "name": "X-Signature"
    }
  }
}
```

Note that if multiple signatures were passed via a comma separated string, each
will be tried unless a match is found. For example:

```
X-Hub-Signature: sha256=the-first-signature,sha256=the-second-signature
```

### 5. Match payload-hash-sha512
```json
{
  "match":
  {
    "type": "payload-hash-sha512",
    "secret": "yoursecret",
    "parameter":
    {
      "source": "header",
      "name": "X-Signature"
    }
  }
}
```

Note that if multiple signatures were passed via a comma separated string, each
will be tried unless a match is found. For example:

```
X-Hub-Signature: sha512=the-first-signature,sha512=the-second-signature
```

### 6. Match Whitelisted IP range

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

### 7. Match scalr-signature

The trigger rule checks the scalr signature and also checks that the request was signed less than 5 minutes before it was received. 
A unqiue signing key is generated for each webhook endpoint URL you register in Scalr.
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
