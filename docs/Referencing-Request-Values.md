# Referencing request values
There are three types of request values:

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

3. Payload (JSON or form-value encoded)
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

3. XML Payload

    Referencing XML payload parameters is much like the JSON examples above, but XML is more complex.
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

    To access a given `user` tag, you must treat them as an array.
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
