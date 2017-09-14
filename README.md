[![ghit.me](https://ghit.me/badge.svg?repo=adnanh/webhook)](https://ghit.me/repo/adnanh/webhook) [![Join the chat at https://gitter.im/adnanh/webhook](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/adnanh/webhook?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge) [![Flattr this](https://button.flattr.com/flattr-badge-large.png)](https://flattr.com/submit/auto?user_id=adnanh&url=https%3A%2F%2Fwww.github.com%2Fadnanh%2Fwebhook) [Donate via PayPal](https://paypal.me/hookdoo) | [Patreon page](https://www.patreon.com/webhook)


# Hookdoo
<a href="https://www.hookdoo.com/?github"><img src="https://my.hookdoo.com/logo/logo-dark-96.png" alt="hookdoo" align="left" /></a>
 
If you don't have time to waste configuring, hosting, debugging and maintaining your webhook instance, we offer a __SaaS__ solution that has all of the capabilities webhook provides, plus a lot more, and all that packaged in a nice friendly web interface. If you are interested, find out more at [hookdoo website](https://www.hookdoo.com/?ref=github-webhook-readme). If you have any questions, you can contact us at info@hookdoo.com


# What is webhook?
[webhook](https://github.com/adnanh/webhook/) is a lightweight configurable tool written in Go, that allows you to easily create HTTP endpoints (hooks) on your server, which you can use to execute configured commands. You can also pass data from the HTTP request (such as headers, payload or query variables) to your commands. [webhook](https://github.com/adnanh/webhook/) also allows you to specify rules which have to be satisfied in order for the hook to be triggered.

For example, if you're using Github or Bitbucket, you can use [webhook](https://github.com/adnanh/webhook/) to set up a hook that runs a redeploy script for your project on your staging server, whenever you push changes to the master branch of your project.

If you use Mattermost or Slack, you can set up an "Outgoing webhook integration" or "Slash command" to run various commands on your server, which can then report back directly to you or your channels using the "Incoming webhook integrations", or the appropriate response body.

[webhook](https://github.com/adnanh/webhook/) aims to do nothing more than it should do, and that is:
 1. receive the request,
 2. parse the headers, payload and query variables,
 3. check if the specified rules for the hook are satisfied,
 3. and finally, pass the specified arguments to the specified command via
    command line arguments or via environment variables.

Everything else is the responsibility of the command's author.

---

# Getting started
## Installation
### Building from source
To get started, first make sure you've properly set up your [Golang](http://golang.org/doc/install) environment and then run the
```bash
$ go get github.com/adnanh/webhook
```
to get the latest version of the [webhook](https://github.com/adnanh/webhook/).

### Using package manager
#### Debian
If you are using Debian linux ("stretch" or later), you can install webhook using `apt-get install webhook` which will install community packaged version (thanks [@freeekanayaka](https://github.com/freeekanayaka)) from https://packages.debian.org/sid/webhook

### Download prebuilt binaries
Prebuilt binaries for different architectures are available at [GitHub Releases](https://github.com/adnanh/webhook/releases).

## Configuration
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

# CORS Headers
If you want to set CORS headers, you can use the `-header name=value` flag while starting [webhook](https://github.com/adnanh/webhook/) to set the appropriate CORS headers that will be returned with each response.

# Interested in running webhook inside of a Docker container?
You can use [almir/webhook](https://hub.docker.com/r/almir/webhook/) docker image, or create your own (please read [this discussion](https://github.com/adnanh/webhook/issues/63)).

# Examples
Check out [Hook examples page](https://github.com/adnanh/webhook/wiki/Hook-Examples) for more complex examples of hooks.

# Contributing
Any form of contribution is welcome and highly appreciated.

Big thanks to [all the current contributors](https://github.com/adnanh/webhook/graphs/contributors) for their contributions!

# Community Contributions
See the [webhook-contrib][wc] repository for a collections of tools and helpers related to [webhook][w] that have been contributed by the [webhook][w] community.

# Sponsors
## <a href="https://www.browserstack.com/?ref=webhook"><img src="http://www.hajdarevic.net/browserstack.svg" alt="BrowserStack" width="250"/></a>
[BrowserStack](https://www.browserstack.com/?ref=webhook) is a cloud-based cross-browser testing tool that enables developers to test their websites across various browsers on different operating systems and mobile devices, without requiring users to install virtual machines, devices or emulators.


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


[w]: https://github.com/adnanh/webhook
[wc]: https://github.com/adnanh/webhook-contrib
