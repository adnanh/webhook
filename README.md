# Installing
  ```go
  go get github.com/adnanh/webhook
  ```

# Updating
  ```go
  go get -u github.com/adnanh/webhook
  ```
  
# Adding hooks
  Hooks are defined using JSON format. The hooks file must contain an array of JSON formatted hooks. Here is an example of a valid hooks file containing one hook. The hook will be triggered whenever a push to the master branch occurrs.
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
# Trigger rules
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
*Not rule* will evaluate to _true_, if and only if the sub rule evaluate to _false_.
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
*Match rule* will evaluate to _true_, if and only if the payload structure contains the key specified in the `parameter` value, contains same value as specified in the `value` value.
*Please note:* due to technical limitations, _number_ and _boolean_ values in hooks file must be wrapped around with quotes.

```json
{
  "match":
  {
    "parameter": "ref",
    "value": "refs/heads/master"
  }
}
```

It is possible to specify the values deeper in the payload JSON object with the dot operator, and if a value of the specified key is an array, it's possible to index the array values by using the number instead of string as the key, as shown in a following example:
```json
{
  "match":
  {
    "parameter": "commits.0.author.username",
    "value": "adnanh"
  }
}
```
# Running
In your `$GOPATH/bin` directory, you should have `webhook` binary.

Simply running the binary using `./webhook` command, will start the webhook with the default options. That means the webhook will listen on all interfaces on port 9000. It will try to read and parse `hooks.json` file from the same directory where the binary is located, and it will log everything to stdout and the file `webhook.log`.

To override any of these options, you can use the following command line flags:
```bash
-hooks="hooks.json": path to the json file containing defined hooks the webhook should serve
-ip="": ip the webhook server should listen on
-log="webhook.log": path to the log file
-port=9000: port the webhook server should listen on
```
