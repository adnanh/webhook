# Hook examples
This page is still work in progress. Feel free to contribute!

## Incoming Github webhook
```json
[
  {
    "id": "webhook",
    "execute-command": "/home/adnan/redeploy-go-webhook.sh",
    "command-working-directory": "/home/adnan/go",
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
            "type": "payload-hash-sha1",
            "secret": "mysecret",
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

## Incoming Bitbucket webhook

Bitbucket does not pass any secrets back to the webhook.  [Per their documentation](https://confluence.atlassian.com/bitbucket/manage-webhooks-735643732.html#Managewebhooks-trigger_webhookTriggeringwebhooks), in order to verify that the webhook came from Bitbucket you must whitelist the IP range `104.192.143.0/24`:

```json
[
  {
    "id": "webhook",
    "execute-command": "/home/adnan/redeploy-go-webhook.sh",
    "command-working-directory": "/home/adnan/go",
    "pass-arguments-to-command":
    [
      {
        "source": "payload",
        "name": "actor.username"
      }
    ],
    "trigger-rule":
    {
      "match":
      {
        "type": "ip-whitelist",
        "ip-range": "104.192.143.0/24"
      }
    }
  }
]
```

## Incoming Gitlab Webhook
Gitlab provides webhooks for many kinds of events. 
Refer to this URL for example request body content: [gitlab-ce/integrations/webhooks](https://gitlab.com/gitlab-org/gitlab-ce/blob/master/doc/user/project/integrations/webhooks.md)
Values in the request body can be accessed in the command or to the match rule by referencing 'payload' as the source:
```json
[
  {
    "id": "redeploy-webhook",
    "execute-command": "/home/adnan/redeploy-go-webhook.sh",
    "command-working-directory": "/home/adnan/go",
    "pass-arguments-to-command":
    [
      {
        "source": "payload",
        "name": "user_name"
      }
    ],
    "response-message": "Executing redeploy script",
    "trigger-rule":
    {
      "match":
      {
        "type": "value",
        "value": "<YOUR-GENERATED-TOKEN>",
        "parameter":
        {
          "source": "header",
          "name": "X-Gitlab-Token"
        }
      }
    }
  }
]
```

## Incoming Gogs webhook
```json
[
  {
    "id": "webhook",
    "execute-command": "/home/adnan/redeploy-go-webhook.sh",
    "command-working-directory": "/home/adnan/go",
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
            "type": "payload-hash-sha256",
            "secret": "mysecret",
            "parameter":
            {
              "source": "header",
              "name": "X-Gogs-Signature"
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
## Incoming Gitea webhook
```json
[
  {
    "id": "webhook",
    "execute-command": "/home/adnan/redeploy-go-webhook.sh",
    "command-working-directory": "/home/adnan/go",
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
            "type": "value",
            "value": "mysecret",
            "parameter":
            {
              "source": "payload",
              "name": "secret"
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

## Slack slash command
```json
[
  {
    "id": "redeploy-webhook",
    "execute-command": "/home/adnan/redeploy-go-webhook.sh",
    "command-working-directory": "/home/adnan/go",
    "response-message": "Executing redeploy script",
    "trigger-rule":
    {
      "match":
      {
        "type": "value",
        "value": "<YOUR-GENERATED-TOKEN>",
        "parameter":
        {
          "source": "payload",
          "name": "token"
        }
      }
    }
  }
]
```

## A simple webhook with a secret key in GET query

__Not recommended in production due to low security__

`example.com:9000/hooks/simple-one` - won't work  
`example.com:9000/hooks/simple-one?token=42` - will work

```json
[
  {
    "id": "simple-one",
    "execute-command": "/path/to/command.sh",
    "response-message": "Executing simple webhook...",
    "trigger-rule":
    {
      "match":
      {
        "type": "value",
        "value": "42",
        "parameter":
        {
          "source": "url",
          "name": "token"
        }
      }
    }
  }
]
```

## JIRA Webhooks
[Guide by @perfecto25](https://sites.google.com/site/mrxpalmeiras/notes/jira-webhooks)

## Pass File-to-command sample

### Webhook configuration

<pre>
[
  {
    "id": "test-file-webhook",
    "execute-command": "/bin/ls",
    "command-working-directory": "/tmp",
    "pass-file-to-command":
    [
      {
      	"source": "payload",
 	"name": "binary",
      	"envname": "ENV_VARIABLE", // to use $ENV_VARIABLE in execute-command
                                   // if not defined, $HOOK_BINARY will be provided
      	"base64decode": true,      // defaults to false
      }
    ],
    "include-command-output-in-response": true
  }
]
</pre>

### Sample client usage 

Store the following file as `testRequest.json`. 

<pre>
{"binary":"iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAAGXRFWHRTb2Z0d2FyZQBBZG9iZSBJbWFnZVJlYWR5ccllPAAAA2lpVFh0WE1MOmNvbS5hZG9iZS54bXAAAAAAADw/eHBhY2tldCBiZWdpbj0i77u/IiBpZD0iVzVNME1wQ2VoaUh6cmVTek5UY3prYzlkIj8+IDx4OnhtcG1ldGEgeG1sbnM6eD0iYWRvYmU6bnM6bWV0YS8iIHg6eG1wdGs9IkFkb2JlIFhNUCBDb3JlIDUuMC1jMDYwIDYxLjEzNDc3NywgMjAxMC8wMi8xMi0xNzozMjowMCAgICAgICAgIj4gPHJkZjpSREYgeG1sbnM6cmRmPSJodHRwOi8vd3d3LnczLm9yZy8xOTk5LzAyLzIyLXJkZi1zeW50YXgtbnMjIj4gPHJkZjpEZXNjcmlwdGlvbiByZGY6YWJvdXQ9IiIgeG1sbnM6eG1wUmlnaHRzPSJodHRwOi8vbnMuYWRvYmUuY29tL3hhcC8xLjAvcmlnaHRzLyIgeG1sbnM6eG1wTU09Imh0dHA6Ly9ucy5hZG9iZS5jb20veGFwLzEuMC9tbS8iIHhtbG5zOnN0UmVmPSJodHRwOi8vbnMuYWRvYmUuY29tL3hhcC8xLjAvc1R5cGUvUmVzb3VyY2VSZWYjIiB4bWxuczp4bXA9Imh0dHA6Ly9ucy5hZG9iZS5jb20veGFwLzEuMC8iIHhtcFJpZ2h0czpNYXJrZWQ9IkZhbHNlIiB4bXBNTTpEb2N1bWVudElEPSJ4bXAuZGlkOjEzMTA4RDI0QzMxQjExRTBCMzYzRjY1QUQ1Njc4QzFBIiB4bXBNTTpJbnN0YW5jZUlEPSJ4bXAuaWlkOjEzMTA4RDIzQzMxQjExRTBCMzYzRjY1QUQ1Njc4QzFBIiB4bXA6Q3JlYXRvclRvb2w9IkFkb2JlIFBob3Rvc2hvcCBDUzMgV2luZG93cyI+IDx4bXBNTTpEZXJpdmVkRnJvbSBzdFJlZjppbnN0YW5jZUlEPSJ1dWlkOkFDMUYyRTgzMzI0QURGMTFBQUI4QzUzOTBEODVCNUIzIiBzdFJlZjpkb2N1bWVudElEPSJ1dWlkOkM5RDM0OTY2NEEzQ0REMTFCMDhBQkJCQ0ZGMTcyMTU2Ii8+IDwvcmRmOkRlc2NyaXB0aW9uPiA8L3JkZjpSREY+IDwveDp4bXBtZXRhPiA8P3hwYWNrZXQgZW5kPSJyIj8+IBFgEwAAAmJJREFUeNqkk89rE1EQx2d/NNq0xcYYayPYJDWC9ODBsKIgAREjBmvEg2cvHnr05KHQ9iB49SL+/BMEfxBQKHgwCEbTNNIYaqgaoanFJi+rcXezye4689jYkIMIDnx47837zrx583YFx3Hgf0xA6/dJyAkkgUy4vgryAnmNWH9L4EVmotFoKplMHgoGg6PkrFarjXQ6/bFcLj/G5W1E+3NaX4KZeDx+dX5+7kg4HBlmrC6JoiDFYrGhROLM/mp1Y6JSqdCd3/SW0GUqEAjkl5ZyHTSHKBQKnO6a9khD2m5cr91IJBJ1VVWdiM/n6LruNJtNDs3JR3ukIW03SHTHi8iVsbG9I51OG1bW16HVasHQZopDc/JZVgdIQ1o3BmTkEnJXURS/KIpgGAYPkCQJPi0u8uzDKQN0XQPbtgE1MmrHs9nsfSqAEjxCNtHxZHLy4G4smUQgyzL4LzOegDGGp1ucVqsNqKVrpJCM7F4hg6iaZvhqtZrg8XjA4xnAU3XeKLqWaRImoIZeQXVjQO5pYp4xNVirsR1erxer2O4yfa227WCwhtWoJmn7m0h270NxmemFW4706zMm8GCgxBGEASCfhnukIW03iFdQnOPz0LNKp3362JqQzSw4u2LXBe+Bs3xD+/oc1NxN55RiC9fOme0LEQiRf2rBzaKEeJJ37ZWTVunBeGN2WmQjg/DeLTVP89nzAive2dMwlo9bpFVC2xWMZr+A720FVn88fAUb3wDMOjyN7YNc6TvUSHQ4AH6TOUdLL7em68UtWPsJqxgTpgeiLu1EBt1R+Me/mF7CQPTfAgwAGxY2vOTrR3oAAAAASUVORK5CYII="}
</pre>

use then the curl tool to execute a request to the webhook.

<pre>
#!/bin/bash
curl -H "Content-Type:application/json" -X POST -d @testRequest.json \
http://localhost:9000/hooks/test-file-webhook
</pre>

or in a single line, using https://github.com/jpmens/jo to generate the JSON code
<pre>
jo binary=%filename.zip | curl -H "Content-Type:application/json" -X POST -d @- \
http://localhost:9000/hooks/test-file-webhook
</pre>


## Incoming Scalr Webhook
[Guide by @hassanbabaie]
Scalr makes webhook calls based on an event to a configured webhook endpoint (for example Host Down, Host Up). Webhook endpoints are URLs where Scalr will deliver Webhook notifications.  
Scalr assigns a unique signing key for every configured webhook endpoint.
Refer to this URL for information on how to setup the webhook call on the Scalr side: [Scalr Wiki Webhooks](https://scalr-wiki.atlassian.net/wiki/spaces/docs/pages/6193173/Webhooks)
In order to leverage the Signing Key for addtional authentication/security you must configure the trigger rule with a match type of "scalr-signature".

```json
[
    {
        "id": "redeploy-webhook",
        "execute-command": "/home/adnan/redeploy-go-webhook.sh",
        "command-working-directory": "/home/adnan/go",
        "include-command-output-in-response": true,
        "trigger-rule": 
		{
            "match": 
			{
                "type": "scalr-signature",
                "secret": "Scalr-provided signing key"
            }
        },
        "pass-environment-to-command": 
		[
            {
                "envname": "EVENT_NAME",
                "source": "payload",
                "name": "eventName"
            },
            {
                "envname": "SERVER_HOSTNAME",
                "source": "payload",
                "name": "data.SCALR_SERVER_HOSTNAME"
            }
        ]
    }
]

```

## Travis CI webhook
Travis sends webhooks as `payload=<JSON_STRING>`, so the payload needs to be parsed as JSON. Here is an example to run on successful builds of the master branch.

```json
[
  {
    "id": "deploy",
    "execute-command": "/root/my-server/deployment.sh",
    "command-working-directory": "/root/my-server",
    "parse-parameters-as-json": [
      {
        "source": "payload",
        "name": "payload"
      }
    ],
    "trigger-rule":
    {
      "and":
      [
        {
          "match":
          {
            "type": "value",
            "value": "passed",
            "parameter": {
              "name": "payload.state",
              "source": "payload"
            }
          }
        },
        {
          "match":
          {
            "type": "value",
            "value": "master",
            "parameter": {
              "name": "payload.branch",
              "source": "payload"
            }
          }
        }
      ]
    }
  }
]
```

## XML Payload

Given the following payload:

```xml
<app>
  <users>
    <user id="1" name="Jeff" />
    <user id="2" name="Sally" />
  </users>
  <messages>
    <message id="1" from_user="1" to_user="2">Hello!!</message>
  </messages>
</app>
```

```json
[
  {
    "id": "deploy",
    "execute-command": "/root/my-server/deployment.sh",
    "command-working-directory": "/root/my-server",
    "trigger-rule": {
      "and": [
        {
          "match": {
            "type": "value",
            "parameter": {
              "source": "payload",
              "name": "app.users.user.0.-name"
            },
            "value": "Jeff"
          }
        },
        {
          "match": {
            "type": "value",
            "parameter": {
              "source": "payload",
              "name": "app.messages.message.#text"
            },
            "value": "Hello!!"
          }
        },
      ],
    }
  }
]
```

## Multipart Form Data

Example of a [Plex Media Server webhook](https://support.plex.tv/articles/115002267687-webhooks/).
The Plex Media Server will send two parts: payload and thumb.
We only care about the payload part.

```json
[
  {
    "id": "plex",
    "execute-command": "play-command.sh",
    "parse-parameters-as-json": [
      {
        "source": "payload",
        "name": "payload"
      }
    ],
    "trigger-rule":
    {
      "match":
      {
        "type": "value",
        "parameter": {
          "source": "payload",
          "name": "payload.event"
        },
        "value": "media.play"
      }
    }
  }
]

```

Each part of a multipart form data body will have a `Content-Disposition` header.
Some example headers:

```
Content-Disposition: form-data; name="payload"
Content-Disposition: form-data; name="thumb"; filename="thumb.jpg"
```

We key off of the `name` attribute in the `Content-Disposition` value.
