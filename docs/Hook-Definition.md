# Hook definition
Hooks are defined as JSON objects. Please note that in order to be considered valid, a hook object must contain the `id` and `execute-command` properties. All other properties are considered optional.

## Properties (keys)

 * `id` - specifies the ID of your hook. This value is used to create the HTTP endpoint (http://yourserver:port/hooks/your-hook-id)
 * `execute-command` - specifies the command that should be executed when the hook is triggered
 * `command-working-directory` - specifies the working directory that will be used for the script when it's executed
 * `response-message` - specifies the string that will be returned to the hook initiator
 * `response-headers` - specifies the list of headers in format `{"name": "X-Example-Header", "value": "it works"}` that will be returned in HTTP response for the hook
 * `success-http-response-code` - specifies the HTTP status code to be returned upon success
 * `incoming-payload-content-type` - sets the `Content-Type` of the incoming HTTP request (ie. `application/json`); useful when the request lacks a `Content-Type` or sends an erroneous value
 * `http-methods` - a list of allowed HTTP methods, such as `POST` and `GET`
 * `include-command-output-in-response` - boolean whether webhook should wait for the command to finish and return the raw output as a response to the hook initiator. If the command fails to execute or encounters any errors while executing the response will result in 500 Internal Server Error HTTP status code, otherwise the 200 OK status code will be returned.
 * `include-command-output-in-response-on-error` - boolean whether webhook should include command stdout & stderror as a response in failed executions. It only works if `include-command-output-in-response` is set to `true`.
 * `parse-parameters-as-json` - specifies the list of arguments that contain JSON strings. These parameters will be decoded by webhook and you can access them like regular objects in rules and `pass-arguments-to-command`.
 * `pass-arguments-to-command` - specifies the list of arguments that will be passed to the command. Check [Referencing request values page](Referencing-Request-Values.md) to see how to reference the values from the request. If you want to pass a static string value to your command you can specify it as
`{ "source": "string", "name": "argumentvalue" }`
 * `pass-environment-to-command` - specifies the list of arguments that will be passed to the command as environment variables. If you do not specify the `"envname"` field in the referenced value, the hook will be in format "HOOK_argumentname", otherwise "envname" field will be used as it's name. Check [Referencing request values page](Referencing-Request-Values.md) to see how to reference the values from the request. If you want to pass a static string value to your command you can specify it as
`{ "source": "string", "envname": "SOMETHING", "name": "argumentvalue" }`
* `pass-file-to-command` - specifies a list of entries that will be serialized as a file. Incoming [data](Referencing-Request-Values.md) will be serialized in a request-temporary-file (otherwise parallel calls of the hook would lead to concurrent overwritings of the file). The filename to be addressed within the subsequent script is provided via an environment variable. Use `envname` to specify the name of the environment variable. If `envname` is not provided `HOOK_` and the name used to reference the request value are used. Defining `command-working-directory` will store the file relative to this location, if not provided, the systems temporary file directory will be used.  If `base64decode` is true, the incoming binary data will be base 64 decoded prior to storing it into the file. By default the corresponding file will be removed after the webhook exited.
 * `trigger-rule` - specifies the rule that will be evaluated in order to determine should the hook be triggered. Check [Hook rules page](Hook-Rules.md) to see the list of valid rules and their usage
 * `trigger-rule-mismatch-http-response-code` - specifies the HTTP status code to be returned when the trigger rule is not satisfied
 * `pre-hook-command` - specifies the command that will be run before the hook gets invoked.
   * to the STDIN of this command, webhook will pass a JSON string representation of an object with the following properties:
     * `hookID` - ID of the hook that got matched
     * `method` - HTTP(s) method used by the client (i.e. GET, POST, etc...)
     * `URI` - URI which client requested
     * `host` - value of the `Host` header sent by the client
     * `remoteAddr` - client's IP address and port in the `IP:PORT` format
     * `query` - object with query parameters and their respective values
     * `headers` - object with headers and their respective values
     * `base64EncodedBody` - base64 encoded request body
    * Output of this command __MUST__ be valid JSON string which will be parsed by the webhook and accessible using the `context` as source when referencing values. 

## Examples
Check out [Hook examples page](Hook-Examples.md) for more complex examples of hooks.
