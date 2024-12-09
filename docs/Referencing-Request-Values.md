# Referencing request values
There are four types of request values:

1. HTTP Request Header values

    ```json
    {
      "source": "header",
      "name": "Header-Name"
    }
    ```

2. HTTP Query parameters

    ```json
    {
      "source": "url",
      "name": "parameter-name"
    }
    ```

3. HTTP Request parameters

    ```json
    {
      "source": "request",
      "name": "method"
    }
    ```

    ```json
    {
      "source": "request",
      "name": "remote-addr"
    }
    ```

4. Payload (JSON or form-value encoded)
    ```json
    {
      "source": "payload",
      "name": "parameter-name"
    }
    ```

    *Note:* For JSON encoded payload, you can reference nested values using the dot-notation.
    For example, if you have following JSON payload
 
    ```json
    {
      "commits": [
        {
          "commit": {
            "id": 1
          }
        }, {
          "commit": {
            "id": 2
          }
        }
      ]
    }
    ```

    You can reference the first commit id as

    ```json
    {
      "source": "payload",
      "name": "commits.0.commit.id"
    }
    ```

    If the payload contains a key with the specified name "commits.0.commit.id", then the value of that key has priority over the dot-notation referencing.

4. XML Payload

    Referencing XML payload parameters is much like the JSON examples above, but XML is more complex.
    Element attributes are prefixed by a hyphen (`-`).
    Element values are prefixed by a pound (`#`).

    Take the following XML payload:

    ```xml
    <app>
      <users>
        <user id="1" name="Jeff" />
        <user id="2" name="Sally" />
      </users>
      <messages>
        <message id="1" from_user="1" to_user="2">Hello!!</message>
      </messages>
    </app>
    ```

    To access a given `user` element, you must treat them as an array.
    So `app.users.user.0.name` yields `Jeff`.

    Since there's only one `message` tag, it's not treated as an array.
    So `app.messages.message.id` yields `1`.

    To access the text within the `message` tag, you would use: `app.messages.message.#text`.

If you are referencing values for environment, you can use `envname` property to set the name of the environment variable like so
```json
{
  "source": "url",
  "name": "q",
  "envname": "QUERY"
}
``` 
to get the QUERY environment variable set to the `q` parameter passed in the query string.

# Special cases
If you want to pass the entire payload as JSON string to your command you can use
```json
{
  "source": "entire-payload"
}
```

for headers you can use
```json
{
  "source": "entire-headers"
}
```

and for query variables you can use
```json
{
  "source": "entire-query"
}
```

# Using a template
If the above source types do not provide sufficient flexibility for your needs, it is possible to provide a [Go template][tt] to compute the value.  The template _context_ provides access to the headers, query parameters, parsed payload, and the complete request body content.  For clarity, the following examples show the YAML form of the definition rather than JSON, since template strings will often contain double quotes, line breaks, etc. that need to be specially encoded in JSON.

## Examples

Extract a value from the payload, if it is present, otherwise from the query string (this allows for a hook that may be called with either a POST request with the form data in the payload, or a GET request with the same data in the URL):

```yaml
- source: template
  name: |-
    {{- with .Payload.requestId -}}
      {{- . -}}
    {{- else -}}
      {{- .Query.requestId -}}
    {{- end -}}
```

Given the following JSON payload describing multiple commits:

```json
{
  "commits": [
    {
      "commit": {
        "commit-id": 1
      }
    }, {
      "commit": {
        "commit-id": 2
      }
    }
  ]
}
```

this template would generate a semicolon-separated list of all the commit IDs:

```yaml
- source: template
  name: |-
    {{- range $i, $c := .Payload.commits -}}
      {{- if gt $i 0 -}};{{- end -}}
      {{- index $c.commit "commit-id" -}}
    {{- end -}}
```

Here `.Payload.commits` is the array of objects, each of these has a field `commit`, which in turn has a field `commit-id`.  The `range` operator iterates over the commits array, setting `$i` to the (zero-based) index and `$c` to the object.  The template then prints a semicolon if this is not the first iteration, then we extract the `commit` field from that object, then in turn the `commit-id`.  Note how the first level can be extracted with just `$c.commit` because the field name is a valid identifier, but for the second level we must use the `index` function.

To access request _header_ values, use the `.GetHeader` function:

```yaml
- source: template
  name: |-
    {{- .GetHeader "x-request-id" }}:{{ index .Query "app-id" -}}
```

## Template context

The following items are available to templates, in addition to the [standard functions](https://pkg.go.dev/text/template#hdr-Functions) provided by Go:

- `.Payload` - the parsed request payload, which may be JSON, XML or form data.
- `.Query` - the query string parameters from the hook URL.
- `.GetHeader "header-name"` - function that returns the value of the given request header, case-insensitive
- `.ContentType` - the request content type
- `.ID` - request ID assigned by webhook itself
- `.Method` - the HTTP request method (`GET`, `POST`, etc.)
- `.RemoteAddr` - IP address of the client (though this may not be accurate if webhook is behind a [reverse proxy](Hook-Rules.md#match-whitelisted-ip-range))
- `.BodyText` - the complete raw content of the request body, as a string

The following are also available but less frequently needed:

- `.Body` - complete body content, but as a slice of bytes rather than as a string
- `.Headers` - the map of HTTP headers.  Useful if you need to `range` over the headers, but to look up keys directly in this map you must use the canonical form - the `.GetHeader` function performs a case-insensitive lookup.

[tt]: https://golang.org/pkg/text/template/