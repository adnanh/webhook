package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/adnanh/webhook/internal/hook"
	"github.com/adnanh/webhook/internal/middleware"
	"github.com/adnanh/webhook/internal/pidfile"

	"github.com/clbanning/mxj"
	chimiddleware "github.com/go-chi/chi/middleware"
	"github.com/gorilla/mux"
	fsnotify "gopkg.in/fsnotify.v1"
)

const (
	version = "2.6.11"
)

var (
	ip                 = flag.String("ip", "0.0.0.0", "ip the webhook should serve hooks on")
	port               = flag.Int("port", 9000, "port the webhook should serve hooks on")
	verbose            = flag.Bool("verbose", false, "show verbose output")
	logPath            = flag.String("logfile", "", "send log output to a file; implicitly enables verbose logging")
	debug              = flag.Bool("debug", false, "show debug output")
	noPanic            = flag.Bool("nopanic", false, "do not panic if hooks cannot be loaded when webhook is not running in verbose mode")
	hotReload          = flag.Bool("hotreload", false, "watch hooks file for changes and reload them automatically")
	hooksURLPrefix     = flag.String("urlprefix", "hooks", "url prefix to use for served hooks (protocol://yourserver:port/PREFIX/:hook-id)")
	secure             = flag.Bool("secure", false, "use HTTPS instead of HTTP")
	asTemplate         = flag.Bool("template", false, "parse hooks file as a Go template")
	cert               = flag.String("cert", "cert.pem", "path to the HTTPS certificate pem file")
	key                = flag.String("key", "key.pem", "path to the HTTPS certificate private key pem file")
	justDisplayVersion = flag.Bool("version", false, "display webhook version and quit")
	justListCiphers    = flag.Bool("list-cipher-suites", false, "list available TLS cipher suites")
	tlsMinVersion      = flag.String("tls-min-version", "1.2", "minimum TLS version (1.0, 1.1, 1.2, 1.3)")
	tlsCipherSuites    = flag.String("cipher-suites", "", "comma-separated list of supported TLS cipher suites")
	useXRequestID      = flag.Bool("x-request-id", false, "use X-Request-Id header, if present, as request ID")
	xRequestIDLimit    = flag.Int("x-request-id-limit", 0, "truncate X-Request-Id header to limit; default no limit")
	maxMultipartMem    = flag.Int64("max-multipart-mem", 1<<20, "maximum memory in bytes for parsing multipart form data before disk caching")
	setGID             = flag.Int("setgid", 0, "set group ID after opening listening port; must be used with setuid")
	setUID             = flag.Int("setuid", 0, "set user ID after opening listening port; must be used with setgid")
	httpMethods        = flag.String("http-methods", "", `set default allowed HTTP methods (ie. "POST"); separate methods with comma`)
	pidPath            = flag.String("pidfile", "", "create PID file at the given path")

	responseHeaders hook.ResponseHeaders
	hooksFiles      hook.HooksFiles

	loadedHooksFromFiles = make(map[string]hook.Hooks)

	watcher *fsnotify.Watcher
	signals chan os.Signal
	pidFile *pidfile.PIDFile
)

func matchLoadedHook(id string) *hook.Hook {
	for _, hooks := range loadedHooksFromFiles {
		if hook := hooks.Match(id); hook != nil {
			return hook
		}
	}

	return nil
}

func lenLoadedHooks() int {
	sum := 0
	for _, hooks := range loadedHooksFromFiles {
		sum += len(hooks)
	}

	return sum
}

func main() {
	flag.Var(&hooksFiles, "hooks", "path to the json file containing defined hooks the webhook should serve, use multiple times to load from different files")
	flag.Var(&responseHeaders, "header", "response header to return, specified in format name=value, use multiple times to set multiple headers")

	flag.Parse()

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

	if (*setUID != 0 || *setGID != 0) && (*setUID == 0 || *setGID == 0) {
		fmt.Println("error: setuid and setgid options must be used together")
		os.Exit(1)
	}

	if *debug || *logPath != "" {
		*verbose = true
	}

	if len(hooksFiles) == 0 {
		hooksFiles = append(hooksFiles, "hooks.json")
	}

	// logQueue is a queue for log messages encountered during startup. We need
	// to queue the messages so that we can handle any privilege dropping and
	// log file opening prior to writing our first log message.
	var logQueue []string

	addr := fmt.Sprintf("%s:%d", *ip, *port)

	// Open listener early so we can drop privileges.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logQueue = append(logQueue, fmt.Sprintf("error listening on port: %s", err))
		// we'll bail out below
	}

	if *setUID != 0 {
		err := dropPrivileges(*setUID, *setGID)
		if err != nil {
			logQueue = append(logQueue, fmt.Sprintf("error dropping privileges: %s", err))
			// we'll bail out below
		}
	}

	if *logPath != "" {
		file, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			logQueue = append(logQueue, fmt.Sprintf("error opening log file %q: %v", *logPath, err))
			// we'll bail out below
		} else {
			log.SetOutput(file)
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

	// load and parse hooks
	for _, hooksFilePath := range hooksFiles {
		log.Printf("attempting to load hooks from %s\n", hooksFilePath)

		newHooks := hook.Hooks{}

		err := newHooks.LoadFromFile(hooksFilePath, *asTemplate)

		if err != nil {
			log.Printf("couldn't load hooks from file! %+v\n", err)
		} else {
			log.Printf("found %d hook(s) in file\n", len(newHooks))

			for _, hook := range newHooks {
				if matchLoadedHook(hook.ID) != nil {
					log.Fatalf("error: hook with the id %s has already been loaded!\nplease check your hooks file for duplicate hooks ids!\n", hook.ID)
				}
				log.Printf("\tloaded: %s\n", hook.ID)
			}

			loadedHooksFromFiles[hooksFilePath] = newHooks
		}
	}

	newHooksFiles := hooksFiles[:0]
	for _, filePath := range hooksFiles {
		if _, ok := loadedHooksFromFiles[filePath]; ok {
			newHooksFiles = append(newHooksFiles, filePath)
		}
	}

	hooksFiles = newHooksFiles

	if !*verbose && !*noPanic && lenLoadedHooks() == 0 {
		log.SetOutput(os.Stdout)
		log.Fatalln("couldn't load any hooks from file!\naborting webhook execution since the -verbose flag is set to false.\nIf, for some reason, you want webhook to start without the hooks, either use -verbose flag, or -nopanic")
	}

	if *hotReload {
		var err error

		watcher, err = fsnotify.NewWatcher()
		if err != nil {
			log.Fatal("error creating file watcher instance\n", err)
		}
		defer watcher.Close()

		for _, hooksFilePath := range hooksFiles {
			// set up file watcher
			log.Printf("setting up file watcher for %s\n", hooksFilePath)

			err = watcher.Add(hooksFilePath)
			if err != nil {
				log.Print("error adding hooks file to the watcher\n", err)
				return
			}
		}

		go watchForFileChange()
	}

	r := mux.NewRouter()

	r.Use(middleware.RequestID(
		middleware.UseXRequestIDHeaderOption(*useXRequestID),
		middleware.XRequestIDLimitOption(*xRequestIDLimit),
	))
	r.Use(middleware.NewLogger())
	r.Use(chimiddleware.Recoverer)

	if *debug {
		r.Use(middleware.Dumper(log.Writer()))
	}

	// Clean up input
	*httpMethods = strings.ToUpper(strings.ReplaceAll(*httpMethods, " ", ""))

	hooksURL := makeURL(hooksURLPrefix)

	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprint(w, "OK")
	})

	r.HandleFunc(hooksURL, hookHandler)

	// Create common HTTP server settings
	svr := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Serve HTTP
	if !*secure {
		log.Printf("serving hooks on http://%s%s", addr, hooksURL)
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

	log.Printf("serving hooks on https://%s%s", addr, hooksURL)
	log.Print(svr.ServeTLS(ln, *cert, *key))
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetReqID(r.Context())

	log.Printf("[%s] incoming HTTP %s request from %s\n", rid, r.Method, r.RemoteAddr)

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
		log.Printf("[%s] HTTP %s method not allowed for hook %q", rid, r.Method, id)

		return
	}

	log.Printf("[%s] %s got matched\n", rid, id)

	for _, responseHeader := range responseHeaders {
		w.Header().Set(responseHeader.Name, responseHeader.Value)
	}

	var (
		body []byte
		err  error
	)

	// set contentType to IncomingPayloadContentType or header value
	contentType := r.Header.Get("Content-Type")
	if len(matchedHook.IncomingPayloadContentType) != 0 {
		contentType = matchedHook.IncomingPayloadContentType
	}

	isMultipart := strings.HasPrefix(contentType, "multipart/form-data;")

	if !isMultipart {
		body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("[%s] error reading the request body: %+v\n", rid, err)
		}
	}

	// parse headers
	headers := valuesToMap(r.Header)

	// parse query variables
	query := valuesToMap(r.URL.Query())

	// parse body
	var payload map[string]interface{}
	// parse context
	var context map[string]interface{}

	if matchedHook.PreHookCommand != "" {
		// check the command exists
		preHookCommandPath, err := exec.LookPath(matchedHook.PreHookCommand)
		if err != nil {
			// give a last chance, maybe it's a relative path
			preHookCommandPathRelativeToCurrentWorkingDirectory := filepath.Join(matchedHook.CommandWorkingDirectory, matchedHook.PreHookCommand)
			// check the command exists
			preHookCommandPath, err = exec.LookPath(preHookCommandPathRelativeToCurrentWorkingDirectory)
		}

		if err != nil {
			log.Printf("[%s] unable to locate pre-hook command: '%s', %+v\n", rid, matchedHook.PreHookCommand, err)
			// check if parameters specified in pre-hook command by mistake
			if strings.IndexByte(matchedHook.PreHookCommand, ' ') != -1 {
				s := strings.Fields(matchedHook.PreHookCommand)[0]
				log.Printf("[%s] please use a wrapper script to provide arguments to pre-hook command for '%s'\n", rid, s)
			}
		} else {
			preHookCommandStdin := hook.PreHookContext{
				HookID:            matchedHook.ID,
				Method:            r.Method,
				Base64EncodedBody: base64.StdEncoding.EncodeToString(body),
				RemoteAddr:        r.RemoteAddr,
				URI:               r.RequestURI,
				Host:              r.Host,
				Headers:           r.Header,
				Query:             r.URL.Query(),
			}

			if preHookCommandStdinJSONString, err := json.Marshal(preHookCommandStdin); err != nil {
				log.Printf("[%s] unable to encode pre-hook context as JSON string for the pre-hook command: %+v\n", rid, err)
			} else {
				preHookCommand := exec.Command(preHookCommandPath)
				preHookCommand.Dir = matchedHook.CommandWorkingDirectory
				preHookCommand.Env = append(os.Environ())

				if preHookCommandStdinPipe, err := preHookCommand.StdinPipe(); err != nil {
					log.Printf("[%s] unable to acquire stdin pipe for the pre-hook command: %+v\n", rid, err)
				} else {
					_, err := io.WriteString(preHookCommandStdinPipe, string(preHookCommandStdinJSONString))
					preHookCommandStdinPipe.Close()
					if err != nil {
						log.Printf("[%s] unable to write to pre-hook command stdin: %+v\n", rid, err)
					} else {
						log.Printf("[%s] executing pre-hook command %s (%s) using %s as cwd\n", rid, matchedHook.PreHookCommand, preHookCommand.Path, preHookCommand.Dir)

						if preHookCommandOutput, err := preHookCommand.CombinedOutput(); err != nil {
							log.Printf("[%s] unable to execute pre-hook command: %+v\n", rid, err)
						} else {
							JSONDecoder := json.NewDecoder(strings.NewReader(string(preHookCommandOutput)))
							JSONDecoder.UseNumber()

							if err := JSONDecoder.Decode(&context); err != nil {
								log.Printf("[%s] unable to parse pre-hook command output: %+v\npre-hook command output was: %+v\n", rid, err, string(preHookCommandOutput))
							}
						}
					}
				}
			}
		}
	}

	switch {
	case strings.Contains(contentType, "json"):
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()

		err := decoder.Decode(&payload)
		if err != nil {
			log.Printf("[%s] error parsing JSON payload %+v\n", rid, err)
		}

	case strings.Contains(contentType, "x-www-form-urlencoded"):
		fd, err := url.ParseQuery(string(body))
		if err != nil {
			log.Printf("[%s] error parsing form payload %+v\n", rid, err)
		} else {
			payload = valuesToMap(fd)
		}

	case strings.Contains(contentType, "xml"):
		payload, err = mxj.NewMapXmlReader(bytes.NewReader(body))
		if err != nil {
			log.Printf("[%s] error parsing XML payload: %+v\n", rid, err)
		}

	case isMultipart:
		err = r.ParseMultipartForm(*maxMultipartMem)
		if err != nil {
			msg := fmt.Sprintf("[%s] error parsing multipart form: %+v\n", rid, err)
			log.Println(msg)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "Error occurred while parsing multipart form.")
			return
		}

		for k, v := range r.MultipartForm.Value {
			log.Printf("[%s] found multipart form value %q", rid, k)

			if payload == nil {
				payload = make(map[string]interface{})
			}

			// TODO(moorereason): support duplicate, named values
			payload[k] = v[0]
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
				log.Printf("[%s] parsing multipart form file %q as JSON\n", rid, k)

				f, err := v[0].Open()
				if err != nil {
					msg := fmt.Sprintf("[%s] error parsing multipart form file: %+v\n", rid, err)
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
					log.Printf("[%s] error parsing JSON payload file: %+v\n", rid, err)
				}

				if payload == nil {
					payload = make(map[string]interface{})
				}
				payload[k] = part
			}
		}

	default:
		log.Printf("[%s] error parsing body payload due to unsupported content type header: %s\n", rid, contentType)
	}

	// handle hook
	errors := matchedHook.ParseJSONParameters(&headers, &query, &payload, &context)
	for _, err := range errors {
		log.Printf("[%s] error parsing JSON parameters: %s\n", rid, err)
	}

	var ok bool

	if matchedHook.TriggerRule == nil {
		ok = true
	} else {
		ok, err = matchedHook.TriggerRule.Evaluate(&headers, &query, &payload, &context, &body, r.RemoteAddr)
		if err != nil {
			if !hook.IsParameterNodeError(err) {
				msg := fmt.Sprintf("[%s] error evaluating hook: %s", rid, err)
				log.Println(msg)
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, "Error occurred while evaluating hook rules.")
				return
			}

			log.Printf("[%s] %v", rid, err)
		}
	}

	if ok {
		log.Printf("[%s] %s hook triggered successfully\n", rid, matchedHook.ID)

		for _, responseHeader := range matchedHook.ResponseHeaders {
			w.Header().Set(responseHeader.Name, responseHeader.Value)
		}

		if matchedHook.CaptureCommandOutput {
			response, err := handleHook(matchedHook, rid, &headers, &query, &payload, &context, &body)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				if matchedHook.CaptureCommandOutputOnError {
					fmt.Fprint(w, response)
				} else {
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					fmt.Fprint(w, "Error occurred while executing the hook's command. Please check your logs for more details.")
				}
			} else {
				// Check if a success return code is configured for the hook
				if matchedHook.SuccessHttpResponseCode != 0 {
					writeHttpResponseCode(w, rid, matchedHook.ID, matchedHook.SuccessHttpResponseCode)
				}
				fmt.Fprint(w, response)
			}
		} else {
			go handleHook(matchedHook, rid, &headers, &query, &payload, &context, &body)

			// Check if a success return code is configured for the hook
			if matchedHook.SuccessHttpResponseCode != 0 {
				writeHttpResponseCode(w, rid, matchedHook.ID, matchedHook.SuccessHttpResponseCode)
			}

			fmt.Fprint(w, matchedHook.ResponseMessage)
		}
		return
	}

	// Check if a return code is configured for the hook
	if matchedHook.TriggerRuleMismatchHttpResponseCode != 0 {
		writeHttpResponseCode(w, rid, matchedHook.ID, matchedHook.TriggerRuleMismatchHttpResponseCode)
	}

	// if none of the hooks got triggered
	log.Printf("[%s] %s got matched, but didn't get triggered because the trigger rules were not satisfied\n", rid, matchedHook.ID)

	fmt.Fprint(w, "Hook rules were not satisfied.")
}

func handleHook(h *hook.Hook, rid string, headers, query, payload *map[string]interface{}, context *map[string]interface{}, body *[]byte) (string, error) {
	var errors []error

	// check the command exists
	cmdPath, err := exec.LookPath(h.ExecuteCommand)
	if err != nil {
		// give a last chance, maybe is a relative path
		relativeToCwd := filepath.Join(h.CommandWorkingDirectory, h.ExecuteCommand)
		// check the command exists
		cmdPath, err = exec.LookPath(relativeToCwd)
	}

	if err != nil {
		log.Printf("[%s] unable to locate command: '%s'\n", rid, h.ExecuteCommand)

		// check if parameters specified in execute-command by mistake
		if strings.IndexByte(h.ExecuteCommand, ' ') != -1 {
			s := strings.Fields(h.ExecuteCommand)[0]
			log.Printf("[%s] please use 'pass-arguments-to-command' to specify args for '%s'\n", rid, s)
		}

		return "", err
	}

	cmd := exec.Command(cmdPath)
	cmd.Dir = h.CommandWorkingDirectory

	cmd.Args, errors = h.ExtractCommandArguments(headers, query, payload, context)
	for _, err := range errors {
		log.Printf("[%s] error extracting command arguments: %s\n", rid, err)
	}

	var envs []string
	envs, errors = h.ExtractCommandArgumentsForEnv(headers, query, payload, context)

	for _, err := range errors {
		log.Printf("[%s] error extracting command arguments for environment: %s\n", rid, err)
	}

	files, errors := h.ExtractCommandArgumentsForFile(headers, query, payload, context)

	for _, err := range errors {
		log.Printf("[%s] error extracting command arguments for file: %s\n", rid, err)
	}

	for i := range files {
		tmpfile, err := ioutil.TempFile(h.CommandWorkingDirectory, files[i].EnvName)
		if err != nil {
			log.Printf("[%s] error creating temp file [%s]\n", rid, err)
			continue
		}
		log.Printf("[%s] writing env %s file %s", rid, files[i].EnvName, tmpfile.Name())
		if _, err := tmpfile.Write(files[i].Data); err != nil {
			log.Printf("[%s] error writing file %s [%s]\n", rid, tmpfile.Name(), err)
			continue
		}
		if err := tmpfile.Close(); err != nil {
			log.Printf("[%s] error closing file %s [%s]\n", rid, tmpfile.Name(), err)
			continue
		}

		files[i].File = tmpfile
		envs = append(envs, files[i].EnvName+"="+tmpfile.Name())
	}

	cmd.Env = append(os.Environ(), envs...)

	log.Printf("[%s] executing %s (%s) with arguments %q and environment %s using %s as cwd\n", rid, h.ExecuteCommand, cmd.Path, cmd.Args, envs, cmd.Dir)

	out, err := cmd.CombinedOutput()

	log.Printf("[%s] command output: %s\n", rid, out)

	if err != nil {
		log.Printf("[%s] error occurred: %+v\n", rid, err)
	}

	for i := range files {
		if files[i].File != nil {
			log.Printf("[%s] removing file %s\n", rid, files[i].File.Name())
			err := os.Remove(files[i].File.Name())
			if err != nil {
				log.Printf("[%s] error removing file %s [%s]\n", rid, files[i].File.Name(), err)
			}
		}
	}

	log.Printf("[%s] finished handling %s\n", rid, h.ID)

	return string(out), err
}

func writeHttpResponseCode(w http.ResponseWriter, rid string, hookId string, responseCode int) {
	// Check if the given return code is supported by the http package
	// by testing if there is a StatusText for this code.
	if len(http.StatusText(responseCode)) > 0 {
		w.WriteHeader(responseCode)
	} else {
		log.Printf("[%s] %s got matched, but the configured return code %d is unknown - defaulting to 200\n", rid, hookId, responseCode)
	}
}

func reloadHooks(hooksFilePath string) {
	hooksInFile := hook.Hooks{}

	// parse and swap
	log.Printf("attempting to reload hooks from %s\n", hooksFilePath)

	err := hooksInFile.LoadFromFile(hooksFilePath, *asTemplate)

	if err != nil {
		log.Printf("couldn't load hooks from file! %+v\n", err)
	} else {
		seenHooksIds := make(map[string]bool)

		log.Printf("found %d hook(s) in file\n", len(hooksInFile))

		for _, hook := range hooksInFile {
			wasHookIDAlreadyLoaded := false

			for _, loadedHook := range loadedHooksFromFiles[hooksFilePath] {
				if loadedHook.ID == hook.ID {
					wasHookIDAlreadyLoaded = true
					break
				}
			}

			if (matchLoadedHook(hook.ID) != nil && !wasHookIDAlreadyLoaded) || seenHooksIds[hook.ID] {
				log.Printf("error: hook with the id %s has already been loaded!\nplease check your hooks file for duplicate hooks ids!", hook.ID)
				log.Println("reverting hooks back to the previous configuration")
				return
			}

			seenHooksIds[hook.ID] = true
			log.Printf("\tloaded: %s\n", hook.ID)
		}

		loadedHooksFromFiles[hooksFilePath] = hooksInFile
	}
}

func reloadAllHooks() {
	for _, hooksFilePath := range hooksFiles {
		reloadHooks(hooksFilePath)
	}
}

func removeHooks(hooksFilePath string) {
	for _, hook := range loadedHooksFromFiles[hooksFilePath] {
		log.Printf("\tremoving: %s\n", hook.ID)
	}

	newHooksFiles := hooksFiles[:0]
	for _, filePath := range hooksFiles {
		if filePath != hooksFilePath {
			newHooksFiles = append(newHooksFiles, filePath)
		}
	}

	hooksFiles = newHooksFiles

	removedHooksCount := len(loadedHooksFromFiles[hooksFilePath])

	delete(loadedHooksFromFiles, hooksFilePath)

	log.Printf("removed %d hook(s) that were loaded from file %s\n", removedHooksCount, hooksFilePath)

	if !*verbose && !*noPanic && lenLoadedHooks() == 0 {
		log.SetOutput(os.Stdout)
		log.Fatalln("couldn't load any hooks from file!\naborting webhook execution since the -verbose flag is set to false.\nIf, for some reason, you want webhook to run without the hooks, either use -verbose flag, or -nopanic")
	}
}

func watchForFileChange() {
	for {
		select {
		case event := <-(*watcher).Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Printf("hooks file %s modified\n", event.Name)
				reloadHooks(event.Name)
			} else if event.Op&fsnotify.Remove == fsnotify.Remove {
				if _, err := os.Stat(event.Name); os.IsNotExist(err) {
					log.Printf("hooks file %s removed, no longer watching this file for changes, removing hooks that were loaded from it\n", event.Name)
					(*watcher).Remove(event.Name)
					removeHooks(event.Name)
				}
			} else if event.Op&fsnotify.Rename == fsnotify.Rename {
				time.Sleep(100 * time.Millisecond)
				if _, err := os.Stat(event.Name); os.IsNotExist(err) {
					// file was removed
					log.Printf("hooks file %s removed, no longer watching this file for changes, and removing hooks that were loaded from it\n", event.Name)
					(*watcher).Remove(event.Name)
					removeHooks(event.Name)
				} else {
					// file was overwritten
					log.Printf("hooks file %s overwritten\n", event.Name)
					reloadHooks(event.Name)
					(*watcher).Remove(event.Name)
					(*watcher).Add(event.Name)
				}
			}
		case err := <-(*watcher).Errors:
			log.Println("watcher error:", err)
		}
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

// makeURL builds a hook URL with or without a prefix.
func makeURL(prefix *string) string {
	if prefix == nil || *prefix == "" {
		return "/{id}"
	}
	return "/" + *prefix + "/{id}"
}
