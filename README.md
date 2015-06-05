[![Join the chat at https://gitter.im/adnanh/webhook](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/adnanh/webhook?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

# What is webhook?
[webhook](https://github.com/adnanh/webhook/) is a lightweight configurable tool written in Go, that allows you to easily create HTTP endpoints (hooks) on your server, which you can use to execute configured commands. You can also pass data from the HTTP request (such as headers, payload or query variables) to your commands. [webhook](https://github.com/adnanh/webhook/) also allows you to specify rules which have to be satisfied in order for the hook to be triggered.

For example, if you're using Github or Bitbucket, you can use [webhook](https://github.com/adnanh/webhook/) to set up a hook that runs a redeploy script for your project on your staging server, whenever you push changes to the master branch of your project.

If you use Slack, you can set up an "Outgoing webhook integration" to run various commands on your server, which can then report back directly to your Slack channels using the "Incoming webhook integrations".

[webhook](https://github.com/adnanh/webhook/) aims to do nothing more than it should do, and that is:
 1. receive the request,
 2. parse the headers, payload and query variables,
 3. check if the specified rules for the hook are satisfied,
 3. and finally, pass the specified arguments to the specified command.

Everything else is the responsibility of the command's author.

---

# Getting started
To get started, first make sure you've properly set up your [Golang](http://golang.org/doc/install) environment and then run the
```bash
$ go get github.com/adnanh/webhook
```
to get the latest version of the [webhook](https://github.com/adnanh/webhook/).

Next step is to define some hooks you want [webhook](https://github.com/adnanh/webhook/) to serve. Begin by creating an empty file named `hooks.json`. This file will contain an array of hooks the [webhook](https://github.com/adnanh/webhook/) will serve. Check [Hook definition page](https://github.com/adnanh/webhook/wiki/Hook-Definition) to see the detailed description of what properties a hook can contain, and how to use them.

Let's define a simple hook named `redeploy-webhook` that will run a redeploy script located in `/var/scripts/redeploy.sh`.

Our `hooks.json` file will now look like this:
```json
[
  {
    "id": "redeploy-webhook",
    "execute-command": "/var/scripts/redeploy.sh",
    "command-working-directory": "/var/webhook"
  }
]
```

You can now run [webhook](https://github.com/adnanh/webhook/) using
```bash
$ /path/to/webhook -hooks hooks.json -verbose
```

It will start up on default port 9000 and will provide you with one HTTP endpoint
```http
http://yourserver:9000/hooks/redeploy-webhook
```

Check [webhook parameters page](https://github.com/adnanh/webhook/wiki/Webhook-Parameters) to see how to override the ip, port and other settings such as hook hotreload, verbose output, etc, when starting the [webhook](https://github.com/adnanh/webhook/).

By performing a simple HTTP GET or POST request to that endpoint, your specified redeploy script would be executed. Neat!

However, hook defined like that could pose a security threat to your system, because anyone who knows your endpoint, can send a request and execute your command. To prevent that, you can use the `"trigger-rule"` property for your hook, to specify the exact circumstances under which the hook would be triggered. For example, you can use them to add a secret that you must supply as a parameter in order to successfully trigger the hook. Please check out the [Hook rules page](https://github.com/adnanh/webhook/wiki/Hook-Rules) for detailed list of available rules and their  usage.

# Using HTTPS
[webhook](https://github.com/adnanh/webhook/) by default serves hooks using http. If you want [webhook](https://github.com/adnanh/webhook/) to serve secure content using https, you can use the `-secure` flag while starting [webhook](https://github.com/adnanh/webhook/). Files containing a certificate and matching private key for the server must be provided using the `-cert /path/to/cert.pem` and `-key /path/to/key.pem` flags. If the certificate is signed by a certificate authority, the cert file should be the concatenation of the server's certificate followed by the CA's certificate.

# Examples
Check out [Hook examples page](https://github.com/adnanh/webhook/wiki/Hook-Examples) for more complex examples of hooks.

# Contributing
Any form of contribution is welcome and highly appreciated.

Big thanks to [all the current contributors](https://github.com/adnanh/webhook/graphs/contributors) for their contributions!

# License

The MIT License (MIT)

Copyright (c) 2015 Adnan Hajdarevic <adnanh@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
