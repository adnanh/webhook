# Hook Examples

Hooks are defined in a hooks configuration file in either JSON or YAML format,
although the examples on this page all use the JSON format.

🌱 This page is still a work in progress. Feel free to contribute!

### Table of Contents

* [Incoming Github webhook](#incoming-github-webhook)
* [Incoming Bitbucket webhook](#incoming-bitbucket-webhook)
* [Incoming Gitlab webhook](#incoming-gitlab-webhook)
* [Incoming Gogs webhook](#incoming-gogs-webhook)
* [Incoming Gitea webhook](#incoming-gitea-webhook)
* [Slack slash command](#slack-slash-command)
* [A simple webhook with a secret key in GET query](#a-simple-webhook-with-a-secret-key-in-get-query)
* [JIRA Webhooks](#jira-webhooks)
* [Pass File-to-command sample](#pass-file-to-command-sample)
* [Incoming Scalr Webhook](#incoming-scalr-webhook)
* [Travis CI webhook](#travis-ci-webhook)
* [XML Payload](#xml-payload)
* [Multipart Form Data](#multipart-form-data)
* [Pass string arguments to command](#pass-string-arguments-to-command)
* [Receive Synology DSM notifications](#receive-synology-notifications)
* [Incoming Azure Container Registry (ACR) webhook](#incoming-acr-webhook)

## Incoming Github webhook

This example works on 2.8+ versions of Webhook - if you are on a previous series, change `payload-hmac-sha1` to `payload-hash-sha1`.

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
            "type": "payload-hmac-sha1",
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

Bitbucket does not pass any secrets back to the webhook.  [Per their documentation](https://support.atlassian.com/organization-administration/docs/ip-addresses-and-domains-for-atlassian-cloud-products/#Outgoing-Connections), in order to verify that the webhook came from Bitbucket you must whitelist a set of IP ranges:

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
      "or":
      [
        { "match": { "type": "ip-whitelist", "ip-range": "13.52.5.96/28" } },
        { "match": { "type": "ip-whitelist", "ip-range": "13.236.8.224/28" } },
        { "match": { "type": "ip-whitelist", "ip-range": "18.136.214.96/28" } },
        { "match": { "type": "ip-whitelist", "ip-range": "18.184.99.224/28" } },
        { "match": { "type": "ip-whitelist", "ip-range": "18.234.32.224/28" } },
        { "match": { "type": "ip-whitelist", "ip-range": "18.246.31.224/28" } },
        { "match": { "type": "ip-whitelist", "ip-range": "52.215.192.224/28" } },
        { "match": { "type": "ip-whitelist", "ip-range": "104.192.137.240/28" } },
        { "match": { "type": "ip-whitelist", "ip-range": "104.192.138.240/28" } },
        { "match": { "type": "ip-whitelist", "ip-range": "104.192.140.240/28" } },
        { "match": { "type": "ip-whitelist", "ip-range": "104.192.142.240/28" } },
        { "match": { "type": "ip-whitelist", "ip-range": "104.192.143.240/28" } },
        { "match": { "type": "ip-whitelist", "ip-range": "185.166.143.240/28" } },
        { "match": { "type": "ip-whitelist", "ip-range": "185.166.142.240/28" } }
      ]
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
            "type": "payload-hmac-sha256",
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

JSON version:

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
        "name": "pusher.full_name"
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
            "type": "payload-hmac-sha256",
            "secret": "mysecret",
            "parameter":
            {
              "source": "header",
              "name": "X-Gitea-Signature"
            }
          }
        },
        {
          "match":
          {
            "type": "value",
            "value": "refs/heads/main",
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

YAML version:

```yaml
- id: webhook
  execute-command: /home/adnan/redeploy-go-webhook.sh
  command-working-directory: /home/adnan/go
  pass-arguments-to-command:
    - source: payload
      name: head_commit.id
    - source: payload
      name: pusher.full_name
    - source: payload
      name: pusher.email
  trigger-rule:
    and:
      - match:
          type: payload-hmac-sha256
          secret: mysecret
          parameter:
            source: header
            name: X-Gitea-Signature
      - match:
          type: value
          value: refs/heads/main
          parameter:
            source: payload
            name: ref
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
[Guide by @perfecto25](https://sites.google.com/site/mrxpalmeiras/more/jira-webhooks)

## Pass File-to-command sample

### Webhook configuration

```json
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
```

### Sample client usage 

Store the following file as `testRequest.json`. 

```json
{"binary":"iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAAGXRFWHRTb2Z0d2FyZQBBZG9iZSBJbWFnZVJlYWR5ccllPAAAA2lpVFh0WE1MOmNvbS5hZG9iZS54bXAAAAAAADw/eHBhY2tldCBiZWdpbj0i77u/IiBpZD0iVzVNME1wQ2VoaUh6cmVTek5UY3prYzlkIj8+IDx4OnhtcG1ldGEgeG1sbnM6eD0iYWRvYmU6bnM6bWV0YS8iIHg6eG1wdGs9IkFkb2JlIFhNUCBDb3JlIDUuMC1jMDYwIDYxLjEzNDc3NywgMjAxMC8wMi8xMi0xNzozMjowMCAgICAgICAgIj4gPHJkZjpSREYgeG1sbnM6cmRmPSJodHRwOi8vd3d3LnczLm9yZy8xOTk5LzAyLzIyLXJkZi1zeW50YXgtbnMjIj4gPHJkZjpEZXNjcmlwdGlvbiByZGY6YWJvdXQ9IiIgeG1sbnM6eG1wUmlnaHRzPSJodHRwOi8vbnMuYWRvYmUuY29tL3hhcC8xLjAvcmlnaHRzLyIgeG1sbnM6eG1wTU09Imh0dHA6Ly9ucy5hZG9iZS5jb20veGFwLzEuMC9tbS8iIHhtbG5zOnN0UmVmPSJodHRwOi8vbnMuYWRvYmUuY29tL3hhcC8xLjAvc1R5cGUvUmVzb3VyY2VSZWYjIiB4bWxuczp4bXA9Imh0dHA6Ly9ucy5hZG9iZS5jb20veGFwLzEuMC8iIHhtcFJpZ2h0czpNYXJrZWQ9IkZhbHNlIiB4bXBNTTpEb2N1bWVudElEPSJ4bXAuZGlkOjEzMTA4RDI0QzMxQjExRTBCMzYzRjY1QUQ1Njc4QzFBIiB4bXBNTTpJbnN0YW5jZUlEPSJ4bXAuaWlkOjEzMTA4RDIzQzMxQjExRTBCMzYzRjY1QUQ1Njc4QzFBIiB4bXA6Q3JlYXRvclRvb2w9IkFkb2JlIFBob3Rvc2hvcCBDUzMgV2luZG93cyI+IDx4bXBNTTpEZXJpdmVkRnJvbSBzdFJlZjppbnN0YW5jZUlEPSJ1dWlkOkFDMUYyRTgzMzI0QURGMTFBQUI4QzUzOTBEODVCNUIzIiBzdFJlZjpkb2N1bWVudElEPSJ1dWlkOkM5RDM0OTY2NEEzQ0REMTFCMDhBQkJCQ0ZGMTcyMTU2Ii8+IDwvcmRmOkRlc2NyaXB0aW9uPiA8L3JkZjpSREY+IDwveDp4bXBtZXRhPiA8P3hwYWNrZXQgZW5kPSJyIj8+IBFgEwAAAmJJREFUeNqkk89rE1EQx2d/NNq0xcYYayPYJDWC9ODBsKIgAREjBmvEg2cvHnr05KHQ9iB49SL+/BMEfxBQKHgwCEbTNNIYaqgaoanFJi+rcXezye4689jYkIMIDnx47837zrx583YFx3Hgf0xA6/dJyAkkgUy4vgryAnmNWH9L4EVmotFoKplMHgoGg6PkrFarjXQ6/bFcLj/G5W1E+3NaX4KZeDx+dX5+7kg4HBlmrC6JoiDFYrGhROLM/mp1Y6JSqdCd3/SW0GUqEAjkl5ZyHTSHKBQKnO6a9khD2m5cr91IJBJ1VVWdiM/n6LruNJtNDs3JR3ukIW03SHTHi8iVsbG9I51OG1bW16HVasHQZopDc/JZVgdIQ1o3BmTkEnJXURS/KIpgGAYPkCQJPi0u8uzDKQN0XQPbtgE1MmrHs9nsfSqAEjxCNtHxZHLy4G4smUQgyzL4LzOegDGGp1ucVqsNqKVrpJCM7F4hg6iaZvhqtZrg8XjA4xnAU3XeKLqWaRImoIZeQXVjQO5pYp4xNVirsR1erxer2O4yfa227WCwhtWoJmn7m0h270NxmemFW4706zMm8GCgxBGEASCfhnukIW03iFdQnOPz0LNKp3362JqQzSw4u2LXBe+Bs3xD+/oc1NxN55RiC9fOme0LEQiRf2rBzaKEeJJ37ZWTVunBeGN2WmQjg/DeLTVP89nzAive2dMwlo9bpFVC2xWMZr+A720FVn88fAUb3wDMOjyN7YNc6TvUSHQ4AH6TOUdLL7em68UtWPsJqxgTpgeiLu1EBt1R+Me/mF7CQPTfAgwAGxY2vOTrR3oAAAAASUVORK5CYII="}
```

use then the curl tool to execute a request to the webhook.

```sh
#!/bin/bash
curl -H "Content-Type:application/json" -X POST -d @testRequest.json \
http://localhost:9000/hooks/test-file-webhook
```

or in a single line, using https://github.com/jpmens/jo to generate the JSON code
```console
jo binary=%filename.zip | curl -H "Content-Type:application/json" -X POST -d @- \
http://localhost:9000/hooks/test-file-webhook
```


## Incoming Scalr Webhook
[Guide by @hassanbabaie]
Scalr makes webhook calls based on an event to a configured webhook endpoint (for example Host Down, Host Up). Webhook endpoints are URLs where Scalr will deliver Webhook notifications.  
Scalr assigns a unique signing key for every configured webhook endpoint.
Refer to this URL for information on how to setup the webhook call on the Scalr side: [Scalr Wiki Webhooks](https://scalr-wiki.atlassian.net/wiki/spaces/docs/pages/6193173/Webhooks)
In order to leverage the Signing Key for additional authentication/security you must configure the trigger rule with a match type of "scalr-signature".

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

## JSON Array Payload

If the JSON payload is an array instead of an object, `webhook` will process the payload and place it into a "root" object.
Therefore, references to payload values must begin with `root.`.

For example, given the following payload (taken from the Sendgrid Event Webhook documentation):
```json
[
  {
    "email": "example@test.com",
    "timestamp": 1513299569,
    "smtp-id": "<14c5d75ce93.dfd.64b469@ismtpd-555>",
    "event": "processed",
    "category": "cat facts",
    "sg_event_id": "sg_event_id",
    "sg_message_id": "sg_message_id"
  },
  {
    "email": "example@test.com",
    "timestamp": 1513299569,
    "smtp-id": "<14c5d75ce93.dfd.64b469@ismtpd-555>",
    "event": "deferred",
    "category": "cat facts",
    "sg_event_id": "sg_event_id",
    "sg_message_id": "sg_message_id",
    "response": "400 try again later",
    "attempt": "5"
  }
]
```

A reference to the second item in the array would look like this:
```json
[
  {
    "id": "sendgrid",
    "execute-command": "/root/my-server/deployment.sh",
    "command-working-directory": "/root/my-server",
    "trigger-rule": {
      "match": {
        "type": "value",
        "parameter": {
          "source": "payload",
          "name": "root.1.event"
        },
        "value": "deferred"
      }
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

## Pass string arguments to command

To pass simple string arguments to a command, use the `string` parameter source.
The following example will pass two static string parameters ("-e 123123") to the
`execute-command` before appending the `pusher.email` value from the payload:

```json
[
  {
    "id": "webhook",
    "execute-command": "/home/adnan/redeploy-go-webhook.sh",
    "command-working-directory": "/home/adnan/go",
    "pass-arguments-to-command":
    [
      {
        "source": "string",
        "name": "-e"
      },
      {
        "source": "string",
        "name": "123123"
      },
      {
        "source": "payload",
        "name": "pusher.email"
      }
    ]
  }
]
```

## Receive Synology DSM notifications

It's possible to securely receive Synology push notifications via webhooks.
Webhooks feature introduced in DSM 7.x seems to be incomplete & broken, but you can use Synology SMS notification service to push webhooks. To configure SMS notifications on DSM follow instructions found here: https://github.com/ryancurrah/synology-notifications this will allow you to set up everything needed for webhook to accept any and all notifications sent by Synology. During setup an 'api_key' is specified - you can generate your own 32-char string and use it as an authentication mechanism to secure your webhook. Additionally, you can specify what notifications to receive via this method by going and selecting the "SMS" checkboxes under topics of interes in DSM: Control Panel -> Notification -> Rules

```json
[
  {
    "id": "synology",
    "execute-command": "do-something.sh",
    "command-working-directory": "/opt/webhook-linux-amd64/synology",
    "response-message": "Request accepted",
    "pass-arguments-to-command":
    [
      {
        "source": "payload",
        "name": "message"
      }
    ],
    "trigger-rule":
    {
      "match":
      {
        "type": "value",
        "value": "PUT_YOUR_API_KEY_HERE",
        "parameter":
        {
          "source": "header",
          "name": "api_key"
        }
      }
    }
  }
]
```
## Incoming Azure Container Registry (ACR) webhook

ACR can send webhooks on image push events.  The `hooks.json` below will handle those events and pass relevant properties as environment variables to a command.

Here is an example of a working docker webhook container used to handle the webhooks and fill the cache of a local registry: [ACR Harbor local cache feeder](https://github.com/tomdess/registry-cache-feeder).


```json
[
  {
    "id": "acr-push-event",
    "execute-command": "/config/script-acr.sh",
    "command-working-directory": "/config",
    "pass-environment-to-command": 
    [
      {
	"envname": "ACTION",
        "source": "payload",
        "name": "action"
      },
      {
	"envname": "REPO",
        "source": "payload",
        "name": "target.repository"
      },
      {
	"envname": "TAG",
        "source": "payload",
        "name": "target.tag"
      },
      {
	"envname": "DIGEST",
        "source": "payload",
        "name": "target.digest"
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
            "value": "mysecretToken",
            "parameter":
            {
              "source": "header",
              "name": "X-Static-Token"
            }
          }
        },
        {
          "match":
          {
            "type": "value",
            "value": "push",
            "parameter":
            {
              "source": "payload",
              "name": "action"
            }
          }
        }
      ]
    }
  }
]
```
