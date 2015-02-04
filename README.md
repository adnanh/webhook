# webhook

## Installing
*Please note:* Before installing the webhook, make sure you have installed `go` and properly set up your `$GOPATH` environment variable.

```go
$ go get github.com/adnanh/webhook
```

## Updating
```go
$ go get -u github.com/adnanh/webhook
```

## Adding hooks
Hooks are defined using JSON format. The _hooks file_ must contain an array of JSON formatted hooks. Here is an example of a valid _hooks file_ containing only one hook. The hook will be triggered whenever a push to the master branch occurrs.

```json
[
  {
    "id": "hook-1",
    "command": "OS command to be executed when the hook gets triggered",
    "cwd": "current working directory under which the specified command will be executed (optional, defaults to the directory where the binary resides)",
    "secret": "secret key used to compute the hash of the payload (optional)",
    "trigger-rule":
    {
      "match":
      {
        "parameter": "ref",
        "value": "refs/heads/master"
      }
    }
  }
]
```
## Trigger rules
### And
*And rule* will evaluate to _true_, if and only if all of the sub rules evaluate to _true_.
```json
{
"and":
  [
    {
      "match":
      {
        "parameter": "ref",
        "value": "refs/heads/master"
      }
    },
    {
      "match":
      {
        "parameter": "repository.owner.name",
        "value": "adnanh"
      }
    }
  ]
}
```
### Or
*Or rule* will evaluate to _true_, if any of the sub rules evaluate to _true_.
```json
{
"or":
  [
    {
      "match":
      {
        "parameter": "ref",
        "value": "refs/heads/master"
      }
    },
    {
      "match":
      {
        "parameter": "ref",
        "value": "refs/heads/development"
      }
    }
  ]
}
```
### Not
*Not rule* will evaluate to _true_, if and only if the sub rule evaluates to _false_.
```json
{
"not":
  {
    "match":
    {
      "parameter": "ref",
      "value": "refs/heads/master"
    }
  }
}
```
### Match
*Match rule* will evaluate to _true_, if and only if the payload JSON object contains the key specified in the `parameter` field that has the same value as specified in the `value` field.

*Please note:* Due to technical reasons, _number_ and _boolean_ values in the _hooks file_ must be wrapped around with a pair of quotes.

```json
{
  "match":
  {
    "parameter": "repository.id",
    "value": "123456"
  }
}
```

It is possible to specify the values deeper in the payload JSON object with the dot operator, and if a value of the specified key happens to be an array, it's possible to index the array values by using the number instead of a string as the key, which is shown in the following example:
```json
{
  "match":
  {
    "parameter": "commits.0.author.username",
    "value": "adnanh"
  }
}
```
## Running
After installing webhook, in your `$GOPATH/bin` directory you should have `webhook` binary.

By simply running the binary using the `./webhook` command, the webhook will start with the default options.
That means the webhook will listen on _all interfaces_ on port `9000`. It will try to read and parse `hooks.json` file from the same directory where the binary is located, and it will log everything to `stdout` and the file `webhook.log`.

To override any of these options, you can use the following command line flags:
```bash
-hooks="hooks.json": path to the json file containing defined hooks the webhook should serve
-ip="": ip the webhook server should listen on
-log="webhook.log": path to the log file
-port=9000: port the webhook server should listen on
```

All hooks are served under the `http://ip:port/hook/:id`, where the `:id` corresponds to the hook *id* specified in _hooks file_.

Visiting `http://ip:port` will show version, uptime and number of hooks the webhook is serving.

## Todo
* Add support for passing parameters from payload to the command that gets executed as part of the hook
* Add support for ip white/black listing
* Add "match-regex" rule
* ???
