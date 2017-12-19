package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/adnanh/webhook/hook"
)

func TestStaticParams(t *testing.T) {

	spHeaders := make(map[string]interface{})
	spHeaders["User-Agent"] = "curl/7.54.0"
	spHeaders["Accept"] = "*/*"

	// case 1: correct entry
	spHook := &hook.Hook{
		ID:                      "static-params-ok",
		ExecuteCommand:          "/bin/echo",
		CommandWorkingDirectory: "/tmp",
		ResponseMessage:         "success",
		CaptureCommandOutput:    true,
		PassArgumentsToCommand: []hook.Argument{
			hook.Argument{Source: "string", Name: "passed"},
		},
	}

	b := &bytes.Buffer{}
	log.SetOutput(b)

	s, err := handleHook(spHook, "test", &spHeaders, &map[string]interface{}{}, &map[string]interface{}{}, &[]byte{})
	if err != nil {
		t.Fatalf("Unexpected error: %v\n", err)
	}
	matched, _ := regexp.MatchString("(?s).*command output: passed.*static-params-ok", b.String())
	if !matched {
		t.Fatalf("Unexpected log output:\n%s", b)
	}

	// case 2: binary with spaces in its name
	err = os.Symlink("/bin/echo", "/tmp/with space")
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer os.Remove("/tmp/with space")

	spHook = &hook.Hook{
		ID:                      "static-params-name-space",
		ExecuteCommand:          "/tmp/with space",
		CommandWorkingDirectory: "/tmp",
		ResponseMessage:         "success",
		CaptureCommandOutput:    true,
		PassArgumentsToCommand: []hook.Argument{
			hook.Argument{Source: "string", Name: "passed"},
		},
	}

	b = &bytes.Buffer{}
	log.SetOutput(b)

	s, err = handleHook(spHook, "test", &spHeaders, &map[string]interface{}{}, &map[string]interface{}{}, &[]byte{})
	if err != nil {
		t.Fatalf("Unexpected error: %v\n", err)
	}
	matched, _ = regexp.MatchString("(?s)command output: .*static-params-name-space", b.String())
	if !matched {
		t.Fatalf("Unexpected log output:\n%sn", b)
	}

	// case 3: parameters specified in execute-command
	spHook = &hook.Hook{
		ID:                      "static-params-bad",
		ExecuteCommand:          "/bin/echo success",
		CommandWorkingDirectory: "/tmp",
		ResponseMessage:         "success",
		CaptureCommandOutput:    true,
		PassArgumentsToCommand: []hook.Argument{
			hook.Argument{Source: "string", Name: "failed"},
		},
	}

	b = &bytes.Buffer{}
	log.SetOutput(b)

	s, err = handleHook(spHook, "test", &spHeaders, &map[string]interface{}{}, &map[string]interface{}{}, &[]byte{})
	if err == nil {
		t.Fatalf("Error expected, but none returned: %s\n", s)
	}
	matched, _ = regexp.MatchString("(?s)unable to locate command: ..bin.echo success.*use 'pass-arguments-to-command'", b.String())
	if !matched {
		t.Fatalf("Unexpected log output:\n%s\n", b)
	}
}

func TestWebhook(t *testing.T) {
	hookecho, cleanupHookecho := buildHookecho(t)
	defer cleanupHookecho()

	for _, hookTmpl := range []string{"test/hooks.json.tmpl", "test/hooks.yaml.tmpl"} {
		config, cleanupConfig := genConfig(t, hookecho, hookTmpl)
		defer cleanupConfig()

		webhook, cleanupWebhook := buildWebhook(t)
		defer cleanupWebhook()

		ip, port := serverAddress(t)
		args := []string{fmt.Sprintf("-hooks=%s", config), fmt.Sprintf("-ip=%s", ip), fmt.Sprintf("-port=%s", port), "-verbose"}

		cmd := exec.Command(webhook, args...)
		//cmd.Stderr = os.Stderr // uncomment to see verbose output
		cmd.Env = webhookEnv()
		cmd.Args[0] = "webhook"
		if err := cmd.Start(); err != nil {
			t.Fatalf("failed to start webhook: %s", err)
		}
		defer killAndWait(cmd)

		waitForServerReady(t, ip, port)

		for _, tt := range hookHandlerTests {
			url := fmt.Sprintf("http://%s:%s/hooks/%s", ip, port, tt.id)

			req, err := http.NewRequest("POST", url, ioutil.NopCloser(strings.NewReader(tt.body)))
			if err != nil {
				t.Errorf("New request failed: %s", err)
			}

			for k, v := range tt.headers {
				req.Header.Add(k, v)
			}

			var res *http.Response

			if tt.urlencoded == true {
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req.Header.Add("Content-Type", "application/json")
			}

			client := &http.Client{}
			res, err = client.Do(req)
			if err != nil {
				t.Errorf("client.Do failed: %s", err)
			}

			body, err := ioutil.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				t.Errorf("POST %q: failed to ready body: %s", tt.desc, err)
			}

			if res.StatusCode != tt.respStatus || string(body) != tt.respBody {
				t.Errorf("failed %q (id: %s):\nexpected status: %#v, response: %s\ngot status: %#v, response: %s", tt.desc, tt.id, tt.respStatus, tt.respBody, res.StatusCode, body)
			}
		}
	}
}

func buildHookecho(t *testing.T) (bin string, cleanup func()) {
	tmp, err := ioutil.TempDir("", "hookecho-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cleanup == nil {
			os.RemoveAll(tmp)
		}
	}()

	bin = filepath.Join(tmp, "hookecho")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", bin, "test/hookecho.go")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Building hookecho: %v", err)
	}

	return bin, func() { os.RemoveAll(tmp) }
}

func genConfig(t *testing.T, bin string, hookTemplate string) (config string, cleanup func()) {
	tmpl := template.Must(template.ParseFiles(hookTemplate))

	tmp, err := ioutil.TempDir("", "webhook-config-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cleanup == nil {
			os.RemoveAll(tmp)
		}
	}()

	outputBaseName := filepath.Ext(filepath.Ext(hookTemplate))

	path := filepath.Join(tmp, outputBaseName)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Creating config template: %v", err)
	}
	defer file.Close()

	data := struct{ Hookecho string }{filepath.ToSlash(bin)}
	if err := tmpl.Execute(file, data); err != nil {
		t.Fatalf("Executing template: %v", err)
	}

	return path, func() { os.RemoveAll(tmp) }
}

func buildWebhook(t *testing.T) (bin string, cleanup func()) {
	tmp, err := ioutil.TempDir("", "webhook-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cleanup == nil {
			os.RemoveAll(tmp)
		}
	}()

	bin = filepath.Join(tmp, "webhook")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", bin)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Building webhook: %v", err)
	}

	return bin, func() { os.RemoveAll(tmp) }
}

func serverAddress(t *testing.T) (string, string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		ln, err = net.Listen("tcp6", "[::1]:0")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("Failed to split network address: %v", err)
	}
	return host, port
}

func waitForServerReady(t *testing.T, ip, port string) {
	waitForServer(t,
		fmt.Sprintf("http://%v:%v/", ip, port),
		http.StatusNotFound,
		5*time.Second)
}

const pollInterval = 200 * time.Millisecond

func waitForServer(t *testing.T, url string, status int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		res, err := http.Get(url)
		if err != nil {
			continue
		}
		if res.StatusCode == status {
			return
		}
	}
	t.Fatalf("Server failed to respond in %v", timeout)
}

func killAndWait(cmd *exec.Cmd) {
	cmd.Process.Kill()
	cmd.Wait()
}

// webhookEnv returns the process environment without any existing hook
// namespace variables.
func webhookEnv() (env []string) {
	for _, v := range os.Environ() {
		if strings.HasPrefix(v, hook.EnvNamespace) {
			continue
		}
		env = append(env, v)
	}
	return
}

var hookHandlerTests = []struct {
	desc       string
	id         string
	headers    map[string]string
	body       string
	urlencoded bool

	respStatus int
	respBody   string
}{
	{
		"github",
		"github",
		map[string]string{"X-Hub-Signature": "f68df0375d7b03e3eb29b4cf9f9ec12e08f42ff8"},
		`{
			"after":"1481a2de7b2a7d02428ad93446ab166be7793fbb",
			"before":"17c497ccc7cca9c2f735aa07e9e3813060ce9a6a",
			"commits":[
				{
					"added":[

					],
					"author":{
						"email":"lolwut@noway.biz",
						"name":"Garen Torikian",
						"username":"octokitty"
					},
					"committer":{
						"email":"lolwut@noway.biz",
						"name":"Garen Torikian",
						"username":"octokitty"
					},
					"distinct":true,
					"id":"c441029cf673f84c8b7db52d0a5944ee5c52ff89",
					"message":"Test",
					"modified":[
						"README.md"
					],
					"removed":[

					],
					"timestamp":"2013-02-22T13:50:07-08:00",
					"url":"https://github.com/octokitty/testing/commit/c441029cf673f84c8b7db52d0a5944ee5c52ff89"
				},
				{
					"added":[

					],
					"author":{
						"email":"lolwut@noway.biz",
						"name":"Garen Torikian",
						"username":"octokitty"
					},
					"committer":{
						"email":"lolwut@noway.biz",
						"name":"Garen Torikian",
						"username":"octokitty"
					},
					"distinct":true,
					"id":"36c5f2243ed24de58284a96f2a643bed8c028658",
					"message":"This is me testing the windows client.",
					"modified":[
						"README.md"
					],
					"removed":[

					],
					"timestamp":"2013-02-22T14:07:13-08:00",
					"url":"https://github.com/octokitty/testing/commit/36c5f2243ed24de58284a96f2a643bed8c028658"
				},
				{
					"added":[
						"words/madame-bovary.txt"
					],
					"author":{
						"email":"lolwut@noway.biz",
						"name":"Garen Torikian",
						"username":"octokitty"
					},
					"committer":{
						"email":"lolwut@noway.biz",
						"name":"Garen Torikian",
						"username":"octokitty"
					},
					"distinct":true,
					"id":"1481a2de7b2a7d02428ad93446ab166be7793fbb",
					"message":"Rename madame-bovary.txt to words/madame-bovary.txt",
					"modified":[

					],
					"removed":[
						"madame-bovary.txt"
					],
					"timestamp":"2013-03-12T08:14:29-07:00",
					"url":"https://github.com/octokitty/testing/commit/1481a2de7b2a7d02428ad93446ab166be7793fbb"
				}
			],
			"compare":"https://github.com/octokitty/testing/compare/17c497ccc7cc...1481a2de7b2a",
			"created":false,
			"deleted":false,
			"forced":false,
			"head_commit":{
				"added":[
					"words/madame-bovary.txt"
				],
				"author":{
					"email":"lolwut@noway.biz",
					"name":"Garen Torikian",
					"username":"octokitty"
				},
				"committer":{
					"email":"lolwut@noway.biz",
					"name":"Garen Torikian",
					"username":"octokitty"
				},
				"distinct":true,
				"id":"1481a2de7b2a7d02428ad93446ab166be7793fbb",
				"message":"Rename madame-bovary.txt to words/madame-bovary.txt",
				"modified":[

				],
				"removed":[
					"madame-bovary.txt"
				],
				"timestamp":"2013-03-12T08:14:29-07:00",
				"url":"https://github.com/octokitty/testing/commit/1481a2de7b2a7d02428ad93446ab166be7793fbb"
			},
			"pusher":{
				"email":"lolwut@noway.biz",
				"name":"Garen Torikian"
			},
			"ref":"refs/heads/master",
			"repository":{
				"created_at":1332977768,
				"description":"",
				"fork":false,
				"forks":0,
				"has_downloads":true,
				"has_issues":true,
				"has_wiki":true,
				"homepage":"",
				"id":3860742,
				"language":"Ruby",
				"master_branch":"master",
				"name":"testing",
				"open_issues":2,
				"owner":{
					"email":"lolwut@noway.biz",
					"name":"octokitty"
				},
				"private":false,
				"pushed_at":1363295520,
				"size":2156,
				"stargazers":1,
				"url":"https://github.com/octokitty/testing",
				"watchers":1
			}
		}`,
		false,
		http.StatusOK,
		`arg: 1481a2de7b2a7d02428ad93446ab166be7793fbb lolwut@noway.biz
env: HOOK_head_commit.timestamp=2013-03-12T08:14:29-07:00
`,
	},
	{
		"bitbucket", // bitbucket sends their payload using uriencoded params.
		"bitbucket",
		nil,
		`payload={"canon_url": "https://bitbucket.org","commits": [{"author": "marcus","branch": "master","files": [{"file": "somefile.py","type": "modified"}],"message": "Added some more things to somefile.py\n","node": "620ade18607a","parents": ["702c70160afc"],"raw_author": "Marcus Bertrand <marcus@somedomain.com>","raw_node": "620ade18607ac42d872b568bb92acaa9a28620e9","revision": null,"size": -1,"timestamp": "2012-05-30 05:58:56","utctimestamp": "2014-11-07 15:19:02+00:00"}],"repository": {"absolute_url": "/webhook/testing/","fork": false,"is_private": true,"name": "Project X","owner": "marcus","scm": "git","slug": "project-x","website": "https://atlassian.com/"},"user": "marcus"}`,
		true,
		http.StatusOK,
		`success`,
	},
	{
		"gitlab",
		"gitlab",
		map[string]string{"X-Gitlab-Event": "Push Hook"},
		`{
			"object_kind": "push",
			"before": "95790bf891e76fee5e1747ab589903a6a1f80f22",
			"after": "da1560886d4f094c3e6c9ef40349f7d38b5d27d7",
			"ref": "refs/heads/master",
			"user_id": 4,
			"user_name": "John Smith",
			"user_email": "john@example.com",
			"project_id": 15,
			"repository": {
				"name": "Diaspora",
				"url": "git@example.com:mike/diasporadiaspora.git",
				"description": "",
				"homepage": "http://example.com/mike/diaspora",
				"git_http_url":"http://example.com/mike/diaspora.git",
				"git_ssh_url":"git@example.com:mike/diaspora.git",
				"visibility_level":0
			},
			"commits": [
				{
					"id": "b6568db1bc1dcd7f8b4d5a946b0b91f9dacd7327",
					"message": "Update Catalan translation to e38cb41.",
					"timestamp": "2011-12-12T14:27:31+02:00",
					"url": "http://example.com/mike/diaspora/commit/b6568db1bc1dcd7f8b4d5a946b0b91f9dacd7327",
					"author": {
						"name": "Jordi Mallach",
						"email": "jordi@softcatala.org"
					}
				},
				{
					"id": "da1560886d4f094c3e6c9ef40349f7d38b5d27d7",
					"message": "fixed readme",
					"timestamp": "2012-01-03T23:36:29+02:00",
					"url": "http://example.com/mike/diaspora/commit/da1560886d4f094c3e6c9ef40349f7d38b5d27d7",
					"author": {
						"name": "GitLab dev user",
						"email": "gitlabdev@dv6700.(none)"
					}
				}
			],
			"total_commits_count": 4
		}`,
		false,
		http.StatusOK,
		`arg: b6568db1bc1dcd7f8b4d5a946b0b91f9dacd7327 John Smith john@example.com
`,
	},

	{
		"missing-cmd-arg", // missing head_commit.author.email
		"github",
		map[string]string{"X-Hub-Signature": "ab03955b9377f530aa298b1b6d273ae9a47e1e40"},
		`{
			"head_commit":{
				"added":[
					"words/madame-bovary.txt"
				],
				"author":{
					"email":"lolwut@noway.biz",
					"name":"Garen Torikian",
					"username":"octokitty"
				},
				"committer":{
					"email":"lolwut@noway.biz",
					"name":"Garen Torikian",
					"username":"octokitty"
				},
				"distinct":true,
				"id":"1481a2de7b2a7d02428ad93446ab166be7793fbb",
				"message":"Rename madame-bovary.txt to words/madame-bovary.txt",
				"modified":[

				],
				"removed":[
					"madame-bovary.txt"
				],
				"timestamp":"2013-03-12T08:14:29-07:00",
				"url":"https://github.com/octokitty/testing/commit/1481a2de7b2a7d02428ad93446ab166be7793fbb"
			},
			"ref":"refs/heads/master"
		}`,
		false,
		http.StatusOK,
		`arg: 1481a2de7b2a7d02428ad93446ab166be7793fbb lolwut@noway.biz
env: HOOK_head_commit.timestamp=2013-03-12T08:14:29-07:00
`,
	},

	{
		"missing-env-arg", // missing head_commit.timestamp
		"github",
		map[string]string{"X-Hub-Signature": "2cf8b878cb6b74a25090a140fa4a474be04b97fa"},
		`{
			"head_commit":{
				"added":[
					"words/madame-bovary.txt"
				],
				"author":{
					"email":"lolwut@noway.biz",
					"name":"Garen Torikian",
					"username":"octokitty"
				},
				"committer":{
					"email":"lolwut@noway.biz",
					"name":"Garen Torikian",
					"username":"octokitty"
				},
				"distinct":true,
				"id":"1481a2de7b2a7d02428ad93446ab166be7793fbb",
				"message":"Rename madame-bovary.txt to words/madame-bovary.txt",
				"modified":[

				],
				"removed":[
					"madame-bovary.txt"
				],
				"url":"https://github.com/octokitty/testing/commit/1481a2de7b2a7d02428ad93446ab166be7793fbb"
			},
			"ref":"refs/heads/master"
		}`,
		false,
		http.StatusOK,
		`arg: 1481a2de7b2a7d02428ad93446ab166be7793fbb lolwut@noway.biz
`,
	},

	// test with custom return code
	{"empty payload", "github", nil, `{}`, false, http.StatusBadRequest, `Hook rules were not satisfied.`},
	// test with custom invalid http code, should default to 200 OK
	{"empty payload", "bitbucket", nil, `{}`, false, http.StatusOK, `Hook rules were not satisfied.`},
	// test with no configured http return code, should default to 200 OK
	{"empty payload", "gitlab", nil, `{}`, false, http.StatusOK, `Hook rules were not satisfied.`},

	// test capturing command output
	{"don't capture output on success by default", "capture-command-output-on-success-not-by-default", nil, `{}`, false, http.StatusOK, ``},
	{"capture output on success with flag set", "capture-command-output-on-success-yes-with-flag", nil, `{}`, false, http.StatusOK, `arg: exit=0
`},
	{"don't capture output on error by default", "capture-command-output-on-error-not-by-default", nil, `{}`, false, http.StatusInternalServerError, `Error occurred while executing the hook's command. Please check your logs for more details.`},
	{"capture output on error with extra flag set", "capture-command-output-on-error-yes-with-extra-flag", nil, `{}`, false, http.StatusInternalServerError, `arg: exit=1
`},
}
