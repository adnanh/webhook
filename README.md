# What is webhook? ![build-status][badge]

 <img src="./docs/logo/logo-128x128.png" alt="Webhook" align="left" />
 
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

# Getting started
## Installation
### Building from source
To get started, first make sure you've properly set up your [Go](http://golang.org/doc/install) 1.19 or newer environment and then run
```bash
$ go build github.com/adnanh/webhook
```
to build the latest version of the [webhook][w].

### Download prebuilt binaries
Prebuilt binaries for different architectures are available at [GitHub Releases](https://github.com/soulteary/webhook/releases).

## Configuration
Next step is to define some hooks you want [webhook][w] to serve.
[webhook][w] supports JSON or YAML configuration files, but we'll focus primarily on JSON in the following example.
Begin by creating an empty file named `hooks.json`. This file will contain an array of hooks the [webhook][w] will serve. Check [Hook definition page](docs/Hook-Definition.md) to see the detailed description of what properties a hook can contain, and how to use them.

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

**NOTE:** If you prefer YAML, the equivalent `hooks.yaml` file would be:
```yaml
- id: redeploy-webhook
  execute-command: "/var/scripts/redeploy.sh"
  command-working-directory: "/var/webhook"
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
[webhook][w] can parse the hooks configuration file as a Go template when given the `-template` [CLI parameter](docs/Webhook-Parameters.md). See the [Templates page](docs/Templates.md) for more details on template usage.

## Using HTTPS
[webhook][w] by default serves hooks using http. If you want [webhook][w] to serve secure content using https, you can use the `-secure` flag while starting [webhook][w]. Files containing a certificate and matching private key for the server must be provided using the `-cert /path/to/cert.pem` and `-key /path/to/key.pem` flags. If the certificate is signed by a certificate authority, the cert file should be the concatenation of the server's certificate followed by the CA's certificate.

TLS version and cipher suite selection flags are available from the command line. To list available cipher suites, use the `-list-cipher-suites` flag.  The `-tls-min-version` flag can be used with `-list-cipher-suites`.

## CORS Headers
If you want to set CORS headers, you can use the `-header name=value` flag while starting [webhook][w] to set the appropriate CORS headers that will be returned with each response.

## Interested in running webhook inside of a Docker container?
You can use one of the following Docker images, or create your own (please read [this discussion](https://github.com/adnanh/webhook/issues/63)):
- [almir/webhook](https://github.com/almir/docker-webhook)
- [roxedus/webhook](https://github.com/Roxedus/docker-webhook)
- [thecatlady/webhook](https://github.com/thecatlady/docker-webhook)

## Examples
Check out [Hook examples page](docs/Hook-Examples.md) for more complex examples of hooks.

### Guides featuring webhook
 - [Plex 2 Telegram](https://gitlab.com/-/snippets/1972594) by [@psyhomb](https://github.com/psyhomb)
 - [Webhook & JIRA](https://sites.google.com/site/mrxpalmeiras/more/jira-webhooks) by [@perfecto25](https://github.com/perfecto25)
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
 - [Set up Automated Deployments From Github With Webhook](https://maximorlov.com/automated-deployments-from-github-with-webhook/) by [Maxim Orlov](https://twitter.com/_maximization)
 - VIDEO: [Gitlab CI/CD configuration using Docker and adnanh/webhook to deploy on VPS - Tutorial #1](https://www.youtube.com/watch?v=Qhn-lXjyrZA&feature=youtu.be) by [Yes! Let's Learn Software Engineering](https://www.youtube.com/channel/UCH4XJf2BZ_52fbf8fOBMF3w)
 - [Integrate automatic deployment in 20 minutes using webhooks + Nginx setup](https://anksus.me/blog/integrate-automatic-deployment-in-20-minutes-using-webhooks) by [Anksus](https://github.com/Anksus)
 - [Automatically redeploy your static blog with Gitea, Uberspace & Webhook](https://by.arran.nz/posts/code/webhook-deploy/) by [Arran](https://arran.nz)
 - ...
 - Want to add your own? Open an Issue or create a PR :-)
 
## Community Contributions
See the [webhook-contrib][wc] repository for a collections of tools and helpers related to [webhook][w] that have been contributed by the [webhook][w] community.

## Need help?
Check out [existing issues](https://github.com/soulteary/webhook/issues) to see if someone else also had the same problem, or [open a new one](https://github.com/soulteary/webhook/issues/new).

# License

The MIT License (MIT)

Copyright (c) 2023 soulteary <soulteary@gmail.com>
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


[w]: https://github.com/soulteary/webhook
[wc]: https://github.com/soulteary/webhook-contrib
[badge]: https://github.com/soulteary/webhook/workflows/build/badge.svg
