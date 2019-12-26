# What is webhook?

 <img src="https://github.com/adnanh/webhook/raw/development/docs/logo/logo-128x128.png" alt="Webhook" align="left" />
 
 [webhook][w] is a lightweight configurable tool written in Go, that allows you to easily create HTTP endpoints (hooks) on your server, which you can use to execute configured commands. You can also pass data from the HTTP request (such as headers, payload or query variables) to your commands. [webhook][w] also allows you to specify rules which have to be satisfied in order for the hook to be triggered.

For example, if you're using Github or Bitbucket, you can use [webhook][w] to set up a hook that runs a redeploy script for your project on your staging server, whenever you push changes to the master branch of your project.

If you use Mattermost or Slack, you can set up an "Outgoing webhook integration" or "Slash command" to run various commands on your server, which can then report back directly to you or your channels using the "Incoming webhook integrations", or the appropriate response body.

[webhook][w] aims to do nothing more than it should do, and that is:
 1. receive the request,
 2. parse the headers, payload and query variables,
 3. check if the specified rules for the hook are satisfied,
 3. and finally, pass the specified arguments to the specified command via
    command line arguments or via environment variables.

Everything else is the responsibility of the command's author.

# Hookdoo
<a href="https://www.hookdoo.com/?github"><img src="https://www.hookdoo.com/logo/logo.svg" height="96" alt="hookdoo" align="left" /></a>

If you don't have time to waste configuring, hosting, debugging and maintaining your webhook instance, we offer a __SaaS__ solution that has all of the capabilities webhook provides, plus a lot more, and all that packaged in a nice friendly web interface. If you are interested, find out more at [hookdoo website](https://www.hookdoo.com/?ref=github-webhook-readme). If you have any questions, you can contact us at info@hookdoo.com


# Getting started
## Installation
### Building from source
To get started, first make sure you've properly set up your [Go](http://golang.org/doc/install) 1.12 or newer environment and then run
```bash
$ go get github.com/adnanh/webhook
```
to get the latest version of the [webhook][w].

### Using package manager
#### Snap store
[![Get it from the Snap Store](https://snapcraft.io/static/images/badges/en/snap-store-white.svg)](https://snapcraft.io/webhook)

#### Ubuntu
If you are using Ubuntu linux (17.04 or later), you can install webhook using `sudo apt-get install webhook` which will install community packaged version.

#### Debian
If you are using Debian linux ("stretch" or later), you can install webhook using `sudo apt-get install webhook` which will install community packaged version (thanks [@freeekanayaka](https://github.com/freeekanayaka)) from https://packages.debian.org/sid/webhook

### Download prebuilt binaries
Prebuilt binaries for different architectures are available at [GitHub Releases](https://github.com/adnanh/webhook/releases).

## Configuration
Next step is to define some hooks you want [webhook][w] to serve. Begin by creating an empty file named `hooks.json`. This file will contain an array of hooks the [webhook][w] will serve. Check [Hook definition page](docs/Hook-Definition.md) to see the detailed description of what properties a hook can contain, and how to use them.

Let's define a simple hook named `redeploy-webhook` that will run a redeploy script located in `/var/scripts/redeploy.sh`. Make sure that your bash script has `#!/bin/sh` shebang on top.

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

You can now run [webhook][w] using
```bash
$ /path/to/webhook -hooks hooks.json -verbose
```

It will start up on default port 9000 and will provide you with one HTTP endpoint
```http
http://yourserver:9000/hooks/redeploy-webhook
```

Check [webhook parameters page](docs/Webhook-Parameters.md) to see how to override the ip, port and other settings such as hook hotreload, verbose output, etc, when starting the [webhook][w].

By performing a simple HTTP GET or POST request to that endpoint, your specified redeploy script would be executed. Neat!

However, hook defined like that could pose a security threat to your system, because anyone who knows your endpoint, can send a request and execute your command. To prevent that, you can use the `"trigger-rule"` property for your hook, to specify the exact circumstances under which the hook would be triggered. For example, you can use them to add a secret that you must supply as a parameter in order to successfully trigger the hook. Please check out the [Hook rules page](docs/Hook-Rules.md) for detailed list of available rules and their  usage.

## Multipart Form Data
[webhook][w] provides limited support the parsing of multipart form data.
Multipart form data can contain two types of parts: values and files.
All form _values_ are automatically added to the `payload` scope.
Use the `parse-parameters-as-json` settings to parse a given value as JSON.
All files are ignored unless they match one of the following criteria:

1. The `Content-Type` header is `application/json`.
1. The part is named in the `parse-parameters-as-json` setting.

In either case, the given file part will be parsed as JSON and added to the `payload` map.

## Templates
[webhook][w] can parse the `hooks.json` input file as a Go template when given the `-template` [CLI parameter](docs/Webhook-Parameters.md). See the [Templates page](docs/Templates.md) for more details on template usage.

## Using HTTPS
[webhook][w] by default serves hooks using http. If you want [webhook][w] to serve secure content using https, you can use the `-secure` flag while starting [webhook][w]. Files containing a certificate and matching private key for the server must be provided using the `-cert /path/to/cert.pem` and `-key /path/to/key.pem` flags. If the certificate is signed by a certificate authority, the cert file should be the concatenation of the server's certificate followed by the CA's certificate.

TLS version and cipher suite selection flags are available from the command line. To list available cipher suites, use the `-list-cipher-suites` flag.  The `-tls-min-version` flag can be used with `-list-cipher-suites`.

## CORS Headers
If you want to set CORS headers, you can use the `-header name=value` flag while starting [webhook][w] to set the appropriate CORS headers that will be returned with each response.

## Interested in running webhook inside of a Docker container?
You can use [almir/webhook](https://hub.docker.com/r/almir/webhook/) docker image, or create your own (please read [this discussion](https://github.com/adnanh/webhook/issues/63)).

## Examples
Check out [Hook examples page](docs/Hook-Examples.md) for more complex examples of hooks.

### Guides featuring webhook
 - [Webhook & JIRA](https://sites.google.com/site/mrxpalmeiras/notes/jira-webhooks) by [@perfecto25](https://github.com/perfecto25)
 - [Trigger Ansible AWX job runs on SCM (e.g. git) commit](http://jpmens.net/2017/10/23/trigger-awx-job-runs-on-scm-commit/) by [@jpmens](http://mens.de/)
 - [Deploy using GitHub webhooks](https://davidauthier.wearemd.com/blog/deploy-using-github-webhooks.html) by [@awea](https://davidauthier.wearemd.com)
 - [Setting up Automatic Deployment and Builds Using Webhooks](https://willbrowning.me/setting-up-automatic-deployment-and-builds-using-webhooks/) by [Will Browning](https://willbrowning.me/about/)
 - [Auto deploy your Node.js app on push to GitHub in 3 simple steps](https://webhookrelay.com/blog/2018/07/17/auto-deploy-on-git-push/) by Karolis Rusenas
 - [Automate Static Site Deployments with Salt, Git, and Webhooks](https://www.linode.com/docs/applications/configuration-management/automate-a-static-site-deployment-with-salt/) by [Linode](https://www.linode.com)
 - [Using Prometheus to Automatically Scale WebLogic Clusters on Kubernetes](https://blogs.oracle.com/weblogicserver/using-prometheus-to-automatically-scale-weblogic-clusters-on-kubernetes-v5) by [Marina Kogan](https://blogs.oracle.com/author/9a4fe754-1cc2-4c64-95fc-360642b62927)
 - [Github Pages and Jekyll - A New Platform for LACNIC Labs](https://labs.lacnic.net/a-new-platform-for-lacniclabs/) by [Carlos Martínez Cagnazzo](https://twitter.com/carlosm3011)
 - [How to Deploy React Apps Using Webhooks and Integrating Slack on Ubuntu](https://www.alibabacloud.com/blog/how-to-deploy-react-apps-using-webhooks-and-integrating-slack-on-ubuntu_594116) by Arslan Ud Din Shafiq
 - [Private webhooks](https://ihateithe.re/2018/01/private-webhooks/) by [Thomas](https://ihateithe.re/colophon/)
 - [Adventures in webhooks](https://medium.com/@draketech/adventures-in-webhooks-2d6584501c62) by [Drake](https://medium.com/@draketech)
 - [GitHub pro tips](http://notes.spencerlyon.com/2016/01/04/github-pro-tips/) by [Spencer Lyon](http://notes.spencerlyon.com/)
 - [XiaoMi Vacuum + Amazon Button = Dash Cleaning](https://www.instructables.com/id/XiaoMi-Vacuum-Amazon-Button-Dash-Cleaning/) by [c0mmensal](https://www.instructables.com/member/c0mmensal/)
 - VIDEO: [Gitlab CI/CD configuration using Docker and adnanh/webhook to deploy on VPS - Tutorial #1](https://www.youtube.com/watch?v=Qhn-lXjyrZA&feature=youtu.be) by [Yes! Let's Learn Software Engineering](https://www.youtube.com/channel/UCH4XJf2BZ_52fbf8fOBMF3w)
 - ...
 - Want to add your own? Open an Issue or create a PR :-)
 
## Community Contributions
See the [webhook-contrib][wc] repository for a collections of tools and helpers related to [webhook][w] that have been contributed by the [webhook][w] community.

## Need help?
Check out [existing issues](https://github.com/adnanh/webhook/issues) to see if someone else also had the same problem, or [open a new one](https://github.com/adnanh/webhook/issues/new).

# Support active development

## Sponsors
## <a href="https://www.digitalocean.com/?ref=webhook"><img src="http://www.hajdarevic.net/DO_Logo_Horizontal_Blue.png" alt="DigitalOcean" width="250"/></a>
[DigitalOcean](https://www.digitalocean.com/?ref=webhook) is a simple and robust cloud computing platform, designed for developers.


## <a href="https://www.browserstack.com/?ref=webhook"><img src="http://www.hajdarevic.net/browserstack.svg" alt="BrowserStack" width="250"/></a>
[BrowserStack](https://www.browserstack.com/?ref=webhook) is a cloud-based cross-browser testing tool that enables developers to test their websites across various browsers on different operating systems and mobile devices, without requiring users to install virtual machines, devices or emulators.

---

Support this project by becoming a sponsor. Your logo will show up here with a link to your website.

<a href="https://opencollective.com/webhook/sponsor/0/website" target="_blank"><img src="https://opencollective.com/webhook/sponsor/0/avatar.svg"></a>
<a href="https://opencollective.com/webhook/sponsor/1/website" target="_blank"><img src="https://opencollective.com/webhook/sponsor/1/avatar.svg"></a>
<a href="https://opencollective.com/webhook/sponsor/2/website" target="_blank"><img src="https://opencollective.com/webhook/sponsor/2/avatar.svg"></a>
<a href="https://opencollective.com/webhook/sponsor/3/website" target="_blank"><img src="https://opencollective.com/webhook/sponsor/3/avatar.svg"></a>
<a href="https://opencollective.com/webhook/sponsor/4/website" target="_blank"><img src="https://opencollective.com/webhook/sponsor/4/avatar.svg"></a>
<a href="https://opencollective.com/webhook/sponsor/5/website" target="_blank"><img src="https://opencollective.com/webhook/sponsor/5/avatar.svg"></a>
<a href="https://opencollective.com/webhook/sponsor/6/website" target="_blank"><img src="https://opencollective.com/webhook/sponsor/6/avatar.svg"></a>
<a href="https://opencollective.com/webhook/sponsor/7/website" target="_blank"><img src="https://opencollective.com/webhook/sponsor/7/avatar.svg"></a>
<a href="https://opencollective.com/webhook/sponsor/8/website" target="_blank"><img src="https://opencollective.com/webhook/sponsor/8/avatar.svg"></a>
<a href="https://opencollective.com/webhook/sponsor/9/website" target="_blank"><img src="https://opencollective.com/webhook/sponsor/9/avatar.svg"></a>

## By contributing

This project exists thanks to all the people who contribute. [Contribute!](CONTRIBUTING.md).
<a href="graphs/contributors"><img src="https://opencollective.com/webhook/contributors.svg?width=890" /></a>

## By giving money

 - [OpenCollective Backer](https://opencollective.com/webhook#backer)
 - [OpenCollective Sponsor](https://opencollective.com/webhook#sponsor)
 - [PayPal](https://paypal.me/hookdoo)
 - [Patreon](https://www.patreon.com/webhook)
 - [Faircode](https://faircode.io/product/webhook?utm_source=badge&utm_medium=badgelarge&utm_campaign=webhook)
 - [Flattr](https://flattr.com/submit/auto?user_id=adnanh&url=https%3A%2F%2Fwww.github.com%2Fadnanh%2Fwebhook)

---

Thank you to all our backers!

<a href="https://opencollective.com/webhook#backers" target="_blank"><img src="https://opencollective.com/webhook/backers.svg?width=890"></a>

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
