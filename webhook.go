package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/adnanh/webhook/hook"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/satori/go.uuid"

	fsnotify "gopkg.in/fsnotify.v1"
	"io"
	"bytes"
	"syscall"
	"context"
	"bufio"
)

const (
	version = "2.6.9"
)

var (
	ip                 = flag.String("ip", "0.0.0.0", "ip the webhook should serve hooks on")
	port               = flag.Int("port", 9000, "port the webhook should serve hooks on")
	verbose            = flag.Bool("verbose", false, "show verbose output")
	noPanic            = flag.Bool("nopanic", false, "do not panic if hooks cannot be loaded when webhook is not running in verbose mode")
	hotReload          = flag.Bool("hotreload", false, "watch hooks file for changes and reload them automatically")
	hooksURLPrefix     = flag.String("urlprefix", "hooks", "url prefix to use for served hooks (protocol://yourserver:port/PREFIX/:hook-id)")
	secure             = flag.Bool("secure", false, "use HTTPS instead of HTTP")
	asTemplate         = flag.Bool("template", false, "parse hooks file as a Go template")
	cert               = flag.String("cert", "cert.pem", "path to the HTTPS certificate pem file")
	key                = flag.String("key", "key.pem", "path to the HTTPS certificate private key pem file")
	justDisplayVersion = flag.Bool("version", false, "display webhook version and quit")

	responseHeaders hook.ResponseHeaders
	hooksFiles      hook.HooksFiles

	loadedHooksFromFiles = make(map[string]hook.Hooks)

	watcher *fsnotify.Watcher
	signals chan os.Signal
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

	if len(hooksFiles) == 0 {
		hooksFiles = append(hooksFiles, "hooks.json")
	}

	log.SetPrefix("[webhook] ")
	log.SetFlags(log.Ldate | log.Ltime)

	if !*verbose {
		log.SetOutput(ioutil.Discard)
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
				log.Fatal("error adding hooks file to the watcher\n", err)
			}
		}

		go watchForFileChange()
	}

	l := negroni.NewLogger()

	l.SetFormat("{{.Status}} | {{.Duration}} | {{.Hostname}} | {{.Method}} {{.Path}} \n")

	standardLogger := log.New(os.Stdout, "[webhook] ", log.Ldate|log.Ltime)

	if !*verbose {
		standardLogger.SetOutput(ioutil.Discard)
	}

	l.ALogger = standardLogger

	negroniRecovery := &negroni.Recovery{
		Logger:     l.ALogger,
		PrintStack: true,
		StackAll:   false,
		StackSize:  1024 * 8,
	}

	n := negroni.New(negroniRecovery, l)

	router := mux.NewRouter()

	var hooksURL string

	if *hooksURLPrefix == "" {
		hooksURL = "/{id}"
	} else {
		hooksURL = "/" + *hooksURLPrefix + "/{id}"
	}

	router.HandleFunc(hooksURL, hookHandler)

	n.UseHandler(router)

	if *secure {
		log.Printf("serving hooks on https://%s:%d%s", *ip, *port, hooksURL)
		log.Fatal(http.ListenAndServeTLS(fmt.Sprintf("%s:%d", *ip, *port), *cert, *key, n))
	} else {
		log.Printf("serving hooks on http://%s:%d%s", *ip, *port, hooksURL)
		log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *ip, *port), n))
	}

}

func lineReader(rdr io.Reader, out io.Writer) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(rdr)
		for scanner.Scan() {
			fmt.Fprintf(out, "%s\n", scanner.Text())
		}
		close(done)
	}()
	return done
}

// combinedOutput simply reads two streams until they terminate and returns the result as a string.
func combinedOutput(stdout io.Reader, stderr io.Reader) string {
	outStream := bytes.NewBuffer(nil)

	stdoutdone := lineReader(stdout, outStream)
	stderrdone := lineReader(stderr, outStream)

	// Order doesn't matter here, we just need both to finish
	<-stdoutdone
	<-stderrdone

	return outStream.String()
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	// generate a request id for logging
	rid := uuid.NewV4().String()[:6]

	log.Printf("[%s] incoming HTTP request from %s\n", rid, r.RemoteAddr)

	id := mux.Vars(r)["id"]

	matchedHook := matchLoadedHook(id)

	// Exit early if no hook matches
	if matchedHook == nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Hook not found.")
		return
	}


	log.Printf("[%s] %s got matched\n", rid, id)

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("[%s] error reading the request body. %+v\n", rid, err)
	}

	// parse headers
	headers := valuesToMap(r.Header)

	// parse query variables
	query := valuesToMap(r.URL.Query())

	// parse body
	var payload map[string]interface{}

	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "json") {
		decoder := json.NewDecoder(strings.NewReader(string(body)))
		decoder.UseNumber()

		err := decoder.Decode(&payload)

		if err != nil {
			log.Printf("[%s] error parsing JSON payload %+v\n", rid, err)
		}
	} else if strings.Contains(contentType, "form") {
		fd, err := url.ParseQuery(string(body))
		if err != nil {
			log.Printf("[%s] error parsing form payload %+v\n", rid, err)
		} else {
			payload = valuesToMap(fd)
		}
	}

	// handle hook
	if errors := matchedHook.ParseJSONParameters(&headers, &query, &payload); errors != nil {
		for _, err := range errors {
			log.Printf("[%s] error parsing JSON parameters: %s\n", rid, err)
		}
	}

	if matchedHook.TriggerRule != nil {
		ok, err := matchedHook.TriggerRule.Evaluate(&headers, &query, &payload, &body, r.RemoteAddr)
		if err != nil {
			msg := fmt.Sprintf("[%s] error evaluating hook: %s", rid, err)
			log.Printf(msg)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error occurred while evaluating hook rules.")
			return
		}

		// Trigger rules did not evaluate. Handle an error.
		if !ok {
			// Check if a return code is configured for the hook
			if matchedHook.TriggerRuleMismatchHttpResponseCode != 0 {
				// Check if the configured return code is supported by the http package
				// by testing if there is a StatusText for this code.
				if len(http.StatusText(matchedHook.TriggerRuleMismatchHttpResponseCode)) > 0 {
					w.WriteHeader(matchedHook.TriggerRuleMismatchHttpResponseCode)
				} else {
					log.Printf("[%s] %s got matched, but the configured return code %d is unknown - defaulting to 200\n", rid, matchedHook.ID, matchedHook.TriggerRuleMismatchHttpResponseCode)
				}
			}

			// if none of the hooks got triggered
			log.Printf("[%s] %s got matched, but didn't get triggered because the trigger rules were not satisfied\n", rid, matchedHook.ID)

			fmt.Fprintf(w, "Hook rules were not satisfied.")

			// Bail.
			return
		}
	}

	// Rule evaluated successfully by this point and will be triggered.

	log.Printf("[%s] %s hook triggered successfully\n", rid, matchedHook.ID)

	// if a regular style webhook, use a background context since we want it to run till it's done.
	// if a streaming webhook, we want to enforce it dies if the user disconnects since it's liable to
	// block forever otherwise.
	var ctx context.Context
	if matchedHook.StreamCommandStdout {
		ctx = r.Context()
	} else {
		ctx = context.Background()
	}

	stdoutRdr, stderrRdr, errCh := handleHook(ctx, matchedHook, rid, &headers, &query, &payload, &body)

	if matchedHook.StreamCommandStdout {
		log.Printf("[%s] Hook (%s) is a streaming command hook\n", rid, matchedHook.ID)
		// Collect stderr to avoid blocking processes and emit it as a string
		stderrRdy := make(chan string, 1)
		go func() {
			stderrOut := bytes.NewBuffer(nil)
			n, err := io.Copy(stderrOut, stderrRdr)
			if err != nil {
				log.Printf("[%s] Hook error while collecting stderr\n", rid)
			}
			log.Printf("[%s] Hook logged %d bytes of stderr data\n", rid, n)
			stderrStr := stderrOut.String()
			log.Printf("[%s] command stderr: %s\n", rid, stderrStr)
			stderrRdy <- stderrStr
			close(stderrRdy)
		}()

		// Streaming output should commence as soon as the command execution tries to write any data
		firstByte := make([]byte,1)
		_, fbErr := stdoutRdr.Read(firstByte)
		if fbErr != nil && fbErr != io.EOF {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error occurred while trying to read from the process's first byte. Please check your logs for more details.")
			log.Printf("[%s] Hook error while reading first byte: %v\n", rid, err)
			return
		} else if fbErr == io.EOF {
			log.Printf("[%s] EOF from hook stdout while reading first byte. Waiting for program exit status\n", rid)
			if err := <- errCh; err != nil {
				log.Printf("[%s] Hook (%s) returned an error before the first byte. Collecting stderr and failing.\n", rid, matchedHook.ID)
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				if matchedHook.StreamCommandStderrOnError {
					// Wait for the stderr buffer to finish collecting
					if n, err := fmt.Fprint(w, <- stderrRdy); err != nil {
						msg := fmt.Sprintf("[%s] error while writing user response message (after %d bytes): %s", rid, n, err)
						log.Printf(msg)
						return
					}
				} else {
					fmt.Fprintf(w, "Error occurred while executing the hooks command. Please check your logs for more details.")
				}
				return	// Cannot proceed beyond here
			}
			// early EOF, but program exited successfully so stream as normal.
		}

		// Write user success headers
		for _, responseHeader := range matchedHook.ResponseHeaders {
			w.Header().Set(responseHeader.Name, responseHeader.Value)
		}

		// Got the first byte (or possibly nothing) successfully. Write the success header, then commence
		// streaming.
		w.WriteHeader(http.StatusOK)

		if _, err := w.Write(firstByte); err != nil {
			// Hard fail, client has disconnected or otherwise we can't continue.
			msg := fmt.Sprintf("[%s] error while trying to stream first byte: %s", rid, err)
			log.Printf(msg)
			return
		}

		n, err := io.Copy(w, stdoutRdr)
		if err != nil {
			msg := fmt.Sprintf("[%s] error while streaming command output (after %d bytes): %s", rid, n, err)
			log.Printf(msg)
			return
		}

		msg := fmt.Sprintf("[%s] Streamed %d bytes", rid, n)
		log.Printf(msg)

	} else {
		log.Printf("[%s] Hook (%s) is a conventional command hook\n", rid, matchedHook.ID)
		// Don't break the original API and just combine the streams (specifically, kick off two readers which
		// break on newlines and the emit that data in temporal order to the output buffer.
		out := combinedOutput(stdoutRdr, stderrRdr)

		log.Printf("[%s] command output: %s\n", rid, out)

		err := <-errCh

		log.Printf("[%s] got command execution result: %v", rid, err)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			for _, responseHeader := range matchedHook.ResponseHeaders {
				w.Header().Set(responseHeader.Name, responseHeader.Value)
			}
			w.WriteHeader(http.StatusOK)
		}

		if matchedHook.CaptureCommandOutput {
			if matchedHook.CaptureCommandOutputOnError || err == nil {
				// Send output if send output on error or no error
				if n, err := fmt.Fprint(w, out); err != nil {
					msg := fmt.Sprintf("[%s] error while writing command output (after %d bytes): %s", rid, n, err)
					log.Printf(msg)
					return
				}
			} else if !matchedHook.CaptureCommandOutputOnError && err != nil {
				// Have an error but not allowed to send output - send error message
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				if n, err := fmt.Fprintf(w, "Error occurred while executing the hook's command. Please check your logs for more details."); err != nil {
					msg := fmt.Sprintf("[%s] error while writing error message (after %d bytes): %s", rid, n, err)
					log.Printf(msg)
					return
				}
			}
		} else {
			// Not capturing command output
			if err != nil {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				if n, err := fmt.Fprintf(w, "Error occurred while executing the hook's command. Please check your logs for more details."); err != nil {
					msg := fmt.Sprintf("[%s] error while writing user response message (after %d bytes): %s", rid, n, err)
					log.Printf(msg)
					return
				}
			} else {
				// Ignore all command output and send the response message
				if n, err := fmt.Fprint(w, matchedHook.ResponseMessage); err != nil {
					msg := fmt.Sprintf("[%s] error while writing user response message (after %d bytes): %s", rid, n, err)
					log.Printf(msg)
					return
				}
			}
		}
	}
}

// errDispatch is a helper to non-blockingly send a single error to a waiting channel and then close it
func errDispatch(err error) <-chan error {
	errCh := make(chan error)
	go func() {
		errCh <- err
		close(errCh)
	}()
	return errCh
}

// handleHook sets up and start the hook command, returning readers for stdout and stderr,
// a channel to return the command result on
func handleHook(ctx context.Context, h *hook.Hook, rid string, headers, query, payload *map[string]interface{},
body *[]byte) (io.Reader, io.Reader, <-chan error) {

	var errors []error

	// check the command exists
	cmdPath, err := exec.LookPath(h.ExecuteCommand)
	if err != nil {
		log.Printf("unable to locate command: '%s'", h.ExecuteCommand)

		// check if parameters specified in execute-command by mistake
		if strings.IndexByte(h.ExecuteCommand, ' ') != -1 {
			s := strings.Fields(h.ExecuteCommand)[0]
			log.Printf("use 'pass-arguments-to-command' to specify args for '%s'", s)
		}

		return bytes.NewBufferString(""), bytes.NewBufferString(""), errDispatch(err)
	}

	cmd := exec.Command(cmdPath)
	cmd.Dir = h.CommandWorkingDirectory

	cmd.Args, errors = h.ExtractCommandArguments(headers, query, payload)
	for _, err := range errors {
		log.Printf("[%s] error extracting command arguments: %s\n", rid, err)
	}

	for _, err := range errors {
		log.Printf("[%s] error setting up command pipes: %s\n", rid, err)
	}

	var envs []string
	envs, errors = h.ExtractCommandArgumentsForEnv(headers, query, payload)

	for _, err := range errors {
		log.Printf("[%s] error extracting command arguments for environment: %s\n", rid, err)
	}

	files, errors := h.ExtractCommandArgumentsForFile(headers, query, payload)

	for _, err := range errors {
		log.Printf("[%s] error extracting command arguments for file: %s\n", rid, err)
	}

	for i := range files {
		tmpfile, err := ioutil.TempFile(h.CommandWorkingDirectory, files[i].EnvName)
		if err != nil {
			log.Printf("[%s] error creating temp file [%s]", rid, err)
			continue
		}
		log.Printf("[%s] writing env %s file %s", rid, files[i].EnvName, tmpfile.Name())
		if _, err := tmpfile.Write(files[i].Data); err != nil {
			log.Printf("[%s] error writing file %s [%s]", rid, tmpfile.Name(), err)
			continue
		}
		if err := tmpfile.Close(); err != nil {
			log.Printf("[%s] error closing file %s [%s]", rid, tmpfile.Name(), err)
			continue
		}

		files[i].File = tmpfile
		envs = append(envs, files[i].EnvName+"="+tmpfile.Name())
	}

	cmd.Env = append(os.Environ(), envs...)

	// Setup stdout and stderr pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return bytes.NewBufferString(""), bytes.NewBufferString(""), errDispatch(err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return bytes.NewBufferString(""), bytes.NewBufferString(""), errDispatch(err)
	}

	log.Printf("[%s] executing %s (%s) with arguments %q and environment %s using %s as cwd\n", rid, h.ExecuteCommand, cmd.Path, cmd.Args, envs, cmd.Dir)

	// Attempt to start the command...
	if err := cmd.Start(); err != nil {
		log.Printf("[%s] error occurred on command start: %+v\n", rid, err)
		return bytes.NewBufferString(""), bytes.NewBufferString(""), errDispatch(err)
	}

	// From this point on we need to actually wait to emit the error
	errCh := make(chan error)
	doneCh := make(chan struct{})

	// Spawn a goroutine to wait for the command to end supply errors
	go func() {
		resultErr := cmd.Wait()
		close(doneCh)	// Close the doneCh immediately so handlers exit correctly.
		if resultErr != nil {
			log.Printf("[%s] error occurred: %+v\n", rid, resultErr)
		}

		for i := range files {
			if files[i].File != nil {
				log.Printf("[%s] removing file %s\n", rid, files[i].File.Name())
				err := os.Remove(files[i].File.Name())
				if err != nil {
					log.Printf("[%s] error removing file %s [%s]", rid, files[i].File.Name(), err)
				}
			}
		}

		log.Printf("[%s] finished handling: %s\n", rid, h.ID)

		errCh <- resultErr
		close(errCh)
	}()

	// Spawn a goroutine which checks if the context is ever cancelled, and sends SIGTERM / SIGKILL if it is
	go func() {
		ctxDone := ctx.Done()

		select {
		case <- ctxDone:
			log.Printf("[%s] Context done (request finished) - killing process.", rid)
			// AFAIK this works on Win/Mac/Unix - where does it not work?
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				log.Printf("[%s] error sending SIGTERM to process for %s: %s\n", rid, h.ID, err)
			} else {
				log.Printf("[%s] Context cancelled sending SIGTERM to process for %s\n", rid, h.ID)
			}
		case <- doneCh:
			// Process has exited, this isn't needed anymore.
			return
		}

		// Process may still be alive, so wait the grace period and then send SIGKILL.
		select {
		case <- doneCh:
			// Process exited after timeout - nothing to do.
			return
		case <- time.After( time.Duration(float64(time.Second) * h.StreamCommandKillGraceSecs) ):
			// Timeout beat process exit. Send kill!
			log.Printf("[%s] Sending SIGKILL to process for %s after grace period of %f seconds\n", rid, h.ID, h.StreamCommandKillGraceSecs)
			if err := cmd.Process.Kill(); err != nil {
				log.Printf("[%s] error sending SIGKILL to process for %s: %s\n", rid, h.ID, err)
			} else {
				log.Printf("[%s] Sent SIGKILL to process for %s\n", rid, h.ID)
			}
		}
		// Nothing left to do. Everything should be dead now.
	}()

	return stdout, stderr, errCh
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
