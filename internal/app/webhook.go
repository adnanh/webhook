package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/adnanh/webhook/internal/hook"
	"github.com/adnanh/webhook/internal/middleware"
	"github.com/adnanh/webhook/internal/network"
	"github.com/adnanh/webhook/internal/pidfile"

	"github.com/gorilla/mux"
)

var (
	version = "2.8.3"
)

var (
	ip                 = flag.String("ip", "0.0.0.0", "Webhook 服务监听的 IP 地址")
	port               = flag.Int("port", 9000, "Webhook 服务监听的端口")
	verbose            = flag.Bool("verbose", false, "输出详细日志")
	logPath            = flag.String("logfile", "", "将日志写入指定文件，并自动开启详细日志")
	debug              = flag.Bool("debug", false, "输出调试日志")
	noPanic            = flag.Bool("nopanic", false, "未开启详细日志时，允许在无法加载 Hook 的情况下继续运行")
	hotReload          = flag.Bool("hotreload", false, "监视 Hook 文件变化并自动重新加载")
	statusURLPrefix    = flag.String("status-path", "status", "服务状态接口的 URL 路径")
	hooksURLPrefix     = flag.String("urlprefix", "hooks", "本地 Hook 的 URL 前缀（访问路径为 /前缀/Hook标识）")
	secure             = flag.Bool("secure", false, "使用 HTTPS 提供服务")
	asTemplate         = flag.Bool("template", false, "将 Hook 配置文件作为 Go 模板解析")
	cert               = flag.String("cert", "cert.pem", "HTTPS 证书的 PEM 文件路径")
	key                = flag.String("key", "key.pem", "HTTPS 证书私钥的 PEM 文件路径")
	justDisplayVersion = flag.Bool("version", false, "显示 Webhook 版本后退出")
	justListCiphers    = flag.Bool("list-cipher-suites", false, "列出可用的 TLS 加密套件")
	tlsMinVersion      = flag.String("tls-min-version", "1.2", "最低 TLS 版本（1.0、1.1、1.2、1.3）")
	tlsCipherSuites    = flag.String("cipher-suites", "", "允许使用的 TLS 加密套件，多个值用逗号分隔")
	useXRequestID      = flag.Bool("x-request-id", false, "优先使用请求头 X-Request-Id 作为请求标识")
	xRequestIDLimit    = flag.Int("x-request-id-limit", 0, "请求头 X-Request-Id 的最大长度，超出时截断；0 表示不限制")
	maxMultipartMem    = flag.Int64("max-multipart-mem", 1<<20, "解析 multipart 表单时的内存上限（字节），超出后缓存到磁盘")
	httpMethods        = flag.String("http-methods", "", `默认允许的 HTTP 方法（例如 POST），多个方法用逗号分隔`)
	pidPath            = flag.String("pidfile", "", "在指定路径创建 PID 文件")
	realIPHeader       = flag.String("real-ip-header", "", "从可信反向代理的指定请求头提取客户端真实 IP（例如 X-Real-IP）")
	trustedProxies     = flag.String("trusted-proxies", "", "可信代理的 IP 或 CIDR，多个值用逗号分隔；real-ip-header 仅对这些代理生效")
	accessWhitelist    = flag.String("access-whitelist", "", "允许访问服务的客户端 IP 或 CIDR，多个值用逗号分隔")
	accessBlacklist    = flag.String("access-blacklist", "", "禁止访问服务的客户端 IP 或 CIDR，多个值用逗号分隔")
	maxBodySize        = flag.Int64("max-body-size", 10<<20, "Webhook 请求体大小上限（字节）；0 表示不限制")
	networkConfigPath  = flag.String("network-config", "", "独立 Hub/Agent 组网 YAML 配置路径，优先于总配置中的 network 部分")

	responseHeaders  hook.ResponseHeaders
	hooksFiles       hook.HooksFiles
	hooksDirectories hook.HooksFiles

	loadedHooksFromFiles = make(map[string]hook.Hooks)

	signals chan os.Signal
	pidFile *pidfile.PIDFile
	setUID  = 0
	setGID  = 0
	socket  = ""
	addr    = ""
)

func matchLoadedHook(id string) *hook.Hook {
	loadedHooksMu.RLock()
	defer loadedHooksMu.RUnlock()

	for _, hooksInFile := range loadedHooksFromFiles {
		if matched := hooksInFile.Match(id); matched != nil {
			cloned := cloneHook(*matched)
			return &cloned
		}
	}

	return nil
}

func lenLoadedHooks() int {
	loadedHooksMu.RLock()
	defer loadedHooksMu.RUnlock()

	sum := 0
	for _, hooksInFile := range loadedHooksFromFiles {
		sum += len(hooksInFile)
	}

	return sum
}

// Main runs the command once using process arguments and exits on fatal errors.
// Build metadata is supplied by the executable entrypoint for -ldflags compatibility.
func Main(buildVersion, manifestPublicKey string) {
	version = buildVersion
	updateManifestPublicKey = manifestPublicKey
	if len(os.Args) > 1 && os.Args[1] == "agent" {
		os.Exit(runAgentCommand(os.Args[2:], os.Stdout, os.Stderr))
	}
	if isUpdateCommand(os.Args) {
		os.Exit(runUpdateCommand(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
	}

	flag.Var(&hooksFiles, "hooks", "Hook 配置文件路径（JSON 或 YAML），可重复指定以加载多个文件")
	flag.Var(&hooksDirectories, "hooks-dir", "包含 JSON 或 YAML Hook 配置的目录，可重复指定；自动监视目录变化")
	flag.Var(&responseHeaders, "header", "返回的响应头，格式为 名称=值，可重复指定")

	// register platform-specific flags
	platformFlags()
	flag.Usage = writeMainHelp
	flag.CommandLine.Usage = writeMainHelp

	// Extract -config / -c from os.Args before flag.Parse() sees it.
	configFile = extractConfigFlag()
	if configFile != "" {
		if err := loadConfigFile(configFile); err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
	}

	flag.Parse()

	if err := initTrustedProxies(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	if err := initAccessControl(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	if *maxBodySize < 0 {
		fmt.Println("error: max-body-size must be zero or greater")
		os.Exit(1)
	}

	if err := initExecutionSettings(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	if *justDisplayVersion {
		fmt.Println("webhook version " + version)
		os.Exit(0)
	}

	if *justListCiphers {
		err := writeTLSSupportedCipherStrings(os.Stdout, getTLSMinVersion(*tlsMinVersion))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	var networkConfig *network.Config
	if *networkConfigPath != "" {
		var err error
		networkConfig, err = network.LoadConfig(*networkConfigPath)
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
	} else if len(inlineNetworkConfig) != 0 {
		var err error
		networkConfig, err = network.ParseConfig(inlineNetworkConfig, inlineNetworkBase)
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
	}
	if networkConfig != nil && networkConfig.Mode == "agent" && !networkConfig.Agent.WithWebhook {
		log.SetPrefix("[webhook] ")
		if *logPath != "" {
			file, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
			if err != nil {
				log.Fatal(err)
			}
			defer file.Close()
			log.SetOutput(file)
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := network.RunAgent(ctx, networkConfig.Agent, network.AgentOptions{Version: version}); err != nil {
			log.Fatal(err)
		}
		return
	}
	if networkConfig != nil && networkConfig.Agent != nil && networkConfig.Agent.WithWebhook {
		if err := prepareWebhookAgent(networkConfig.Agent); err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
		if networkConfig.Agent.WebhookOnlyViaHub {
			*realIPHeader = network.ClientIPHeader
			*trustedProxies = "127.0.0.1,::1"
			if err := initTrustedProxies(); err != nil {
				fmt.Println("error:", err)
				os.Exit(1)
			}
		} else if *realIPHeader == "" && *trustedProxies == "" {
			*realIPHeader = network.ClientIPHeader
			*trustedProxies = "127.0.0.1,::1"
			if err := initTrustedProxies(); err != nil {
				fmt.Println("error:", err)
				os.Exit(1)
			}
		}
	}

	if (setUID != 0 || setGID != 0) && (setUID == 0 || setGID == 0) {
		fmt.Println("error: setuid and setgid options must be used together")
		os.Exit(1)
	}

	if *debug || *logPath != "" {
		*verbose = true
	}

	if len(hooksFiles) == 0 && len(hooksDirectories) == 0 && (networkConfig == nil || networkConfig.Agent != nil && networkConfig.Agent.WithWebhook) {
		hooksFiles = append(hooksFiles, "hooks.json")
	}

	if err := initAdmin(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	if err := initStatus(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	// logQueue is a queue for log messages encountered during startup. We need
	// to queue the messages so that we can handle any privilege dropping and
	// log file opening prior to writing our first log message.
	var logQueue []string

	// by default the listen address is ip:port (default 0.0.0.0:9000), but
	// this may be modified by trySocketListener
	addr = fmt.Sprintf("%s:%d", *ip, *port)

	onlyViaHub := networkConfig != nil && networkConfig.Agent != nil && networkConfig.Agent.WebhookOnlyViaHub
	var ln net.Listener
	if !onlyViaHub {
		var err error
		ln, err = trySocketListener()
		if err != nil {
			logQueue = append(logQueue, fmt.Sprintf("error listening on socket: %s", err))
		} else if ln == nil {
			// Open listener early so we can drop privileges.
			ln, err = net.Listen("tcp", addr)
			if err != nil {
				logQueue = append(logQueue, fmt.Sprintf("error listening on port: %s", err))
			}
		}
	}

	if setUID != 0 {
		err := dropPrivileges(setUID, setGID)
		if err != nil {
			logQueue = append(logQueue, fmt.Sprintf("error dropping privileges: %s", err))
			// we'll bail out below
		}
	}

	if *logPath != "" {
		file, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			logQueue = append(logQueue, fmt.Sprintf("error opening log file %q: %v", *logPath, err))
			// we'll bail out below
		} else {
			if err := file.Chmod(0o600); err != nil {
				logQueue = append(logQueue, fmt.Sprintf("error securing log file %q: %v", *logPath, err))
				_ = file.Close()
			} else {
				log.SetOutput(file)
			}
		}
	}

	log.SetPrefix("[webhook] ")
	log.SetFlags(log.Ldate | log.Ltime)

	if len(logQueue) != 0 {
		for i := range logQueue {
			log.Println(logQueue[i])
		}

		os.Exit(1)
	}

	if !*verbose {
		log.SetOutput(ioutil.Discard)
	}

	// Create pidfile
	if *pidPath != "" {
		var err error

		pidFile, err = pidfile.New(*pidPath)
		if err != nil {
			log.Fatalf("Error creating pidfile: %v", err)
		}

		defer func() {
			// NOTE(moorereason): my testing shows that this doesn't work with
			// ^C, so we also do a Remove in the signal handler elsewhere.
			if nerr := pidFile.Remove(); nerr != nil {
				log.Print(nerr)
			}
		}()
	}

	log.Println("version " + version + " starting")

	// set os signal watcher
	setupSignals()

	if err := initializeHookSources(); err != nil {
		log.Fatalf("error initializing hooks sources: %v", err)
	}

	if !*verbose && !*noPanic && len(hooksDirectories) == 0 && lenLoadedHooks() == 0 && (networkConfig == nil || networkConfig.Agent != nil && networkConfig.Agent.WithWebhook) {
		log.SetOutput(os.Stdout)
		log.Fatalln("couldn't load any hooks from file!\naborting webhook execution since the -verbose flag is set to false.\nIf, for some reason, you want webhook to start without the hooks, either use -verbose flag, or -nopanic")
	}

	if *hotReload || len(hooksDirectories) != 0 {
		if err := startHookWatcher(); err != nil {
			log.Fatal("error creating hooks watcher: ", err)
		}
		defer watcher.Close()
		go watchForFileChange()
	}

	// Clean up input
	*httpMethods = strings.ToUpper(strings.ReplaceAll(*httpMethods, " ", ""))

	var hub *network.Hub
	if networkConfig != nil && networkConfig.Hub != nil {
		if err := validateNetworkRoutes(networkConfig.Hub.Routes); err != nil {
			log.Fatal(err)
		}
		hub = network.NewHub(networkConfig.Hub)
		defer hub.Close()
	}
	handler := newHTTPHandler(hub)

	// Create common HTTP server settings
	svr := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if networkConfig != nil && networkConfig.Agent != nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var provider func() []network.HookInfo
		if networkConfig.Agent.WithWebhook {
			provider = networkHookInventory
		}
		options := network.AgentOptions{Version: version, Hooks: provider, Downstream: hub}
		if networkConfig.Agent.WithWebhook {
			options.WebhookHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				prefix := makeBaseURL(hooksURLPrefix) + "/"
				if !strings.HasPrefix(req.URL.Path, prefix) || !network.ValidHookID(strings.TrimPrefix(req.URL.Path, prefix)) {
					http.NotFound(w, req)
					return
				}
				clientIP := net.ParseIP(req.Header.Get(network.ClientIPHeader))
				if clientIP == nil {
					http.Error(w, "invalid network client IP", http.StatusForbidden)
					return
				}
				req = req.WithContext(context.WithValue(req.Context(), networkClientIPKey{}, clientIP.String()))
				handler.ServeHTTP(w, req)
			})
		}
		if onlyViaHub {
			log.Print("serving hooks only through the network hub")
			if err := network.RunAgent(ctx, networkConfig.Agent, options); err != nil {
				log.Printf("network agent stopped: %v", err)
			}
			return
		}
		go func() {
			if err := network.RunAgent(ctx, networkConfig.Agent, options); err != nil {
				log.Printf("network agent stopped: %v", err)
			}
		}()
	}

	// Serve HTTP
	if !*secure {
		log.Printf("serving hooks on http://%s%s", addr, makeHumanPattern(hooksURLPrefix))
		if *adminEnabled {
			log.Printf("serving admin on http://%s%s", addr, currentAdminAuth.basePath)
		}
		log.Print(svr.Serve(ln))

		return
	}

	// Server HTTPS
	svr.TLSConfig = &tls.Config{
		CipherSuites:             getTLSCipherSuites(*tlsCipherSuites),
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		MinVersion:               getTLSMinVersion(*tlsMinVersion),
		PreferServerCipherSuites: true,
	}
	svr.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler)) // disable http/2

	log.Printf("serving hooks on https://%s%s", addr, makeHumanPattern(hooksURLPrefix))
	if *adminEnabled {
		log.Printf("serving admin on https://%s%s", addr, currentAdminAuth.basePath)
	}
	log.Print(svr.ServeTLS(ln, *cert, *key))
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	if *maxBodySize > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, *maxBodySize)
	}

	req := &hook.Request{
		ID:         middleware.GetReqID(r.Context()),
		RawRequest: r,
		RealIP:     resolveRealIP(r),
	}

	log.Printf("[%s] incoming HTTP %s request from %s\n", req.ID, r.Method, req.RealIP)

	// TODO: rename this to avoid confusion with Request.ID
	id := mux.Vars(r)["id"]

	matchedHook := matchLoadedHook(id)
	if matchedHook == nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Hook not found.")
		return
	}

	// Check for allowed methods
	var allowedMethod bool

	switch {
	case len(matchedHook.HTTPMethods) != 0:
		for i := range matchedHook.HTTPMethods {
			// TODO(moorereason): refactor config loading and reloading to
			// sanitize these methods once at load time.
			if r.Method == strings.ToUpper(strings.TrimSpace(matchedHook.HTTPMethods[i])) {
				allowedMethod = true
				break
			}
		}
	case *httpMethods != "":
		for _, v := range strings.Split(*httpMethods, ",") {
			if r.Method == v {
				allowedMethod = true
				break
			}
		}
	default:
		allowedMethod = true
	}

	if !allowedMethod {
		w.WriteHeader(http.StatusMethodNotAllowed)
		log.Printf("[%s] HTTP %s method not allowed for hook %q", req.ID, r.Method, id)

		return
	}

	log.Printf("[%s] %s got matched\n", req.ID, id)

	for _, responseHeader := range responseHeaders {
		w.Header().Set(responseHeader.Name, responseHeader.Value)
	}

	var err error

	// set contentType to IncomingPayloadContentType or header value
	req.ContentType = r.Header.Get("Content-Type")
	if len(matchedHook.IncomingPayloadContentType) != 0 {
		req.ContentType = matchedHook.IncomingPayloadContentType
	}

	isMultipart := strings.HasPrefix(req.ContentType, "multipart/form-data;")

	if !isMultipart {
		req.Body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("[%s] error reading the request body: %+v\n", req.ID, err)
			if *maxBodySize > 0 {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				fmt.Fprint(w, "Request body too large.")
				return
			}
		}
	}

	req.ParseHeaders(r.Header)
	req.ParseQuery(r.URL.Query())

	switch {
	case strings.Contains(req.ContentType, "json"):
		err = req.ParseJSONPayload()
		if err != nil {
			log.Printf("[%s] %s", req.ID, err)
		}

	case strings.Contains(req.ContentType, "x-www-form-urlencoded"):
		err = req.ParseFormPayload()
		if err != nil {
			log.Printf("[%s] %s", req.ID, err)
		}

	case strings.Contains(req.ContentType, "xml"):
		err = req.ParseXMLPayload()
		if err != nil {
			log.Printf("[%s] %s", req.ID, err)
		}

	case isMultipart:
		err = r.ParseMultipartForm(*maxMultipartMem)
		if err != nil {
			msg := fmt.Sprintf("[%s] error parsing multipart form: %+v\n", req.ID, err)
			log.Println(msg)
			if _, ok := err.(*http.MaxBytesError); ok {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				fmt.Fprint(w, "Request body too large.")
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "Error occurred while parsing multipart form.")
			return
		}

		for k, v := range r.MultipartForm.Value {
			log.Printf("[%s] found multipart form value %q", req.ID, k)

			if req.Payload == nil {
				req.Payload = make(map[string]interface{})
			}

			// TODO(moorereason): support duplicate, named values
			req.Payload[k] = v[0]
		}

		for k, v := range r.MultipartForm.File {
			// Force parsing as JSON regardless of Content-Type.
			var parseAsJSON bool
			for _, j := range matchedHook.JSONStringParameters {
				if j.Source == "payload" && j.Name == k {
					parseAsJSON = true
					break
				}
			}

			// TODO(moorereason): we need to support multiple parts
			// with the same name instead of just processing the first
			// one. Will need #215 resolved first.

			// MIME encoding can contain duplicate headers, so check them
			// all.
			if !parseAsJSON && len(v[0].Header["Content-Type"]) > 0 {
				for _, j := range v[0].Header["Content-Type"] {
					if j == "application/json" {
						parseAsJSON = true
						break
					}
				}
			}

			if parseAsJSON {
				log.Printf("[%s] parsing multipart form file %q as JSON\n", req.ID, k)

				f, err := v[0].Open()
				if err != nil {
					msg := fmt.Sprintf("[%s] error parsing multipart form file: %+v\n", req.ID, err)
					log.Println(msg)
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, "Error occurred while parsing multipart form file.")
					return
				}

				decoder := json.NewDecoder(f)
				decoder.UseNumber()

				var part map[string]interface{}
				err = decoder.Decode(&part)
				if err != nil {
					log.Printf("[%s] error parsing JSON payload file: %+v\n", req.ID, err)
				}
				if err := f.Close(); err != nil {
					log.Printf("[%s] error closing multipart form file: %+v\n", req.ID, err)
				}

				if req.Payload == nil {
					req.Payload = make(map[string]interface{})
				}
				req.Payload[k] = part
			}
		}

	default:
		log.Printf("[%s] error parsing body payload due to unsupported content type header: %s\n", req.ID, req.ContentType)
	}

	// handle hook
	errors := matchedHook.ParseJSONParameters(req)
	for _, err := range errors {
		log.Printf("[%s] error parsing JSON parameters: %s\n", req.ID, err)
	}

	var ok bool

	if matchedHook.TriggerRule == nil {
		ok = true
	} else {
		// Save signature soft failures option in request for evaluators
		req.AllowSignatureErrors = matchedHook.TriggerSignatureSoftFailures

		ok, err = matchedHook.TriggerRule.Evaluate(req)
		if err != nil {
			if !hook.IsParameterNodeError(err) {
				msg := fmt.Sprintf("[%s] error evaluating hook: %s", req.ID, err)
				log.Println(msg)
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, "Error occurred while evaluating hook rules.")
				return
			}

			log.Printf("[%s] %v", req.ID, err)
		}
	}

	if ok {
		log.Printf("[%s] %s hook triggered successfully\n", req.ID, matchedHook.ID)

		for _, responseHeader := range matchedHook.ResponseHeaders {
			w.Header().Set(responseHeader.Name, responseHeader.Value)
		}

		if matchedHook.CaptureCommandOutput {
			response, err := handleHook(matchedHook, req)

			if err != nil {
				status, message := hookExecutionErrorStatus(err)
				if matchedHook.CaptureCommandOutputOnError && response != "" {
					w.WriteHeader(status)
					fmt.Fprint(w, response)
				} else {
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					w.WriteHeader(status)
					fmt.Fprint(w, message)
				}
			} else {
				// Check if a success return code is configured for the hook
				if matchedHook.SuccessHttpResponseCode != 0 {
					writeHttpResponseCode(w, req.ID, matchedHook.ID, matchedHook.SuccessHttpResponseCode)
				}
				fmt.Fprint(w, response)
			}
		} else {
			if err := handleHookAsync(matchedHook, req); err != nil {
				status, message := hookExecutionErrorStatus(err)
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(status)
				fmt.Fprint(w, message)
				return
			}

			// Check if a success return code is configured for the hook
			if matchedHook.SuccessHttpResponseCode != 0 {
				writeHttpResponseCode(w, req.ID, matchedHook.ID, matchedHook.SuccessHttpResponseCode)
			}

			fmt.Fprint(w, matchedHook.ResponseMessage)
		}
		return
	}

	// Check if a return code is configured for the hook
	if matchedHook.TriggerRuleMismatchHttpResponseCode != 0 {
		writeHttpResponseCode(w, req.ID, matchedHook.ID, matchedHook.TriggerRuleMismatchHttpResponseCode)
	}

	// if none of the hooks got triggered
	log.Printf("[%s] %s got matched, but didn't get triggered because the trigger rules were not satisfied\n", req.ID, matchedHook.ID)

	fmt.Fprint(w, "Hook rules were not satisfied.")
}

func executeHook(h *hook.Hook, r *hook.Request, waitForCommand bool, release func()) (string, error) {
	defer release()
	if r.RawRequest != nil && r.RawRequest.MultipartForm != nil {
		defer func() {
			if err := r.RawRequest.MultipartForm.RemoveAll(); err != nil && !os.IsNotExist(err) {
				log.Printf("[%s] error removing multipart form temp files [%s]", r.ID, err)
			}
		}()
	}

	var errors []error

	// check the command exists
	var lookpath string
	if filepath.IsAbs(h.ExecuteCommand) || h.CommandWorkingDirectory == "" {
		lookpath = h.ExecuteCommand
	} else {
		lookpath = filepath.Join(h.CommandWorkingDirectory, h.ExecuteCommand)
	}

	cmdPath, err := exec.LookPath(lookpath)
	if err != nil {
		log.Printf("[%s] error in %s", r.ID, err)

		// check if parameters specified in execute-command by mistake
		if strings.IndexByte(h.ExecuteCommand, ' ') != -1 {
			s := strings.Fields(h.ExecuteCommand)[0]
			log.Printf("[%s] use 'pass-arguments-to-command' to specify args for '%s'", r.ID, s)
		}

		return "", err
	}

	ctx, cancel, useContext, err := hookExecutionContext(h, r, waitForCommand)
	if err != nil {
		log.Printf("[%s] error preparing command execution: %s", r.ID, err)
		return "", err
	}
	defer cancel()

	var cmd *exec.Cmd
	if useContext {
		cmd = exec.CommandContext(ctx, cmdPath)
	} else {
		cmd = exec.Command(cmdPath)
	}
	cmd.Dir = h.CommandWorkingDirectory

	cmd.Args, errors = h.ExtractCommandArguments(r)
	for _, err := range errors {
		log.Printf("[%s] error extracting command arguments: %s\n", r.ID, err)
	}

	var envs []string
	envs, errors = h.ExtractCommandArgumentsForEnv(r)

	for _, err := range errors {
		log.Printf("[%s] error extracting command arguments for environment: %s\n", r.ID, err)
	}

	files, errors := h.ExtractCommandArgumentsForFile(r)

	for _, err := range errors {
		log.Printf("[%s] error extracting command arguments for file: %s\n", r.ID, err)
	}

	for i := range files {
		tmpfile, err := ioutil.TempFile(h.CommandWorkingDirectory, files[i].EnvName)
		if err != nil {
			log.Printf("[%s] error creating temp file [%s]", r.ID, err)
			continue
		}
		log.Printf("[%s] writing env %s file %s", r.ID, files[i].EnvName, tmpfile.Name())
		if _, err := tmpfile.Write(files[i].Data); err != nil {
			log.Printf("[%s] error writing file %s [%s]", r.ID, tmpfile.Name(), err)
			continue
		}
		if err := tmpfile.Close(); err != nil {
			log.Printf("[%s] error closing file %s [%s]", r.ID, tmpfile.Name(), err)
			continue
		}

		files[i].File = tmpfile
		envs = append(envs, files[i].EnvName+"="+tmpfile.Name())
	}

	if h.KeepFileEnvironment && r.RawRequest != nil && r.RawRequest.MultipartForm != nil {
		for k, v := range r.RawRequest.MultipartForm.File {
			env_name := hook.EnvNamespace + "FILE_" + strings.ToUpper(k)
			f, err := v[0].Open()
			if err != nil {
				log.Printf("[%s] error open form %s file [%s]", r.ID, k, err)
				continue
			}
			if f1, ok := f.(*os.File); ok {
				log.Printf("[%s] temporary file %s", r.ID, f1.Name())
				_ = f1.Close()
				files = append(files, hook.FileParameter{File: f1, EnvName: env_name})
				envs = append(envs,
					env_name+"="+f1.Name(),
					hook.EnvNamespace+"FILENAME_"+strings.ToUpper(k)+"="+v[0].Filename,
				)
				continue
			}
			tmpfile, err := os.CreateTemp("", ".hook-"+r.ID+"-"+k+"-*")
			if err != nil {
				_ = f.Close()
				log.Printf("[%s] error creating temp file [%s]", r.ID, err)
				continue
			}
			log.Printf("[%s] writing env %s file %s", r.ID, env_name, tmpfile.Name())
			if _, err = io.Copy(tmpfile, f); err != nil {
				log.Printf("[%s] error writing file %s [%s]", r.ID, tmpfile.Name(), err)
				_ = f.Close()
				_ = tmpfile.Close()
				_ = os.Remove(tmpfile.Name())
				continue
			}
			if err := tmpfile.Close(); err != nil {
				log.Printf("[%s] error closing file %s [%s]", r.ID, tmpfile.Name(), err)
				_ = os.Remove(tmpfile.Name())
				continue
			}
			_ = f.Close()
			files = append(files, hook.FileParameter{File: tmpfile, EnvName: env_name})
			envs = append(envs,
				env_name+"="+tmpfile.Name(),
				hook.EnvNamespace+"FILENAME_"+strings.ToUpper(k)+"="+v[0].Filename,
			)
		}
	}

	cmd.Env = append(os.Environ(), envs...)

	log.Printf("[%s] executing %s (%s) with arguments %q and environment %s using %s as cwd\n", r.ID, h.ExecuteCommand, cmd.Path, cmd.Args, envs, cmd.Dir)

	out, err := cmd.CombinedOutput()

	log.Printf("[%s] command output: %s\n", r.ID, out)

	if err != nil {
		log.Printf("[%s] error occurred: %+v\n", r.ID, err)
	}

	for i := range files {
		if files[i].File != nil {
			log.Printf("[%s] removing file %s\n", r.ID, files[i].File.Name())
			err := os.Remove(files[i].File.Name())
			if err != nil && !os.IsNotExist(err) {
				log.Printf("[%s] error removing file %s [%s]", r.ID, files[i].File.Name(), err)
			}
		}
	}

	log.Printf("[%s] finished handling %s\n", r.ID, h.ID)

	return string(out), err
}

func writeHttpResponseCode(w http.ResponseWriter, rid, hookId string, responseCode int) {
	// Check if the given return code is supported by the http package
	// by testing if there is a StatusText for this code.
	if len(http.StatusText(responseCode)) > 0 {
		w.WriteHeader(responseCode)
	} else {
		log.Printf("[%s] %s got matched, but the configured return code %d is unknown - defaulting to 200\n", rid, hookId, responseCode)
	}
}

// valuesToMap converts map[string][]string to a map[string]string object
func valuesToMap(values map[string][]string) map[string]interface{} {
	ret := make(map[string]interface{})

	for key, value := range values {
		if len(value) > 0 {
			ret[key] = value[0]
		}
	}

	return ret
}

// makeRoutePattern builds a pattern matching URL for the mux.
func makeRoutePattern(prefix *string) string {
	return makeBaseURL(prefix) + "/{id:.*}"
}

// makeHumanPattern builds a human-friendly URL for display.
func makeHumanPattern(prefix *string) string {
	return makeBaseURL(prefix) + "/{id}"
}

// makeBaseURL creates the base URL before any mux pattern matching.
func makeBaseURL(prefix *string) string {
	if prefix == nil || *prefix == "" {
		return ""
	}

	return "/" + *prefix
}
