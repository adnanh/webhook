# Referencing request values
There are four types of request values:

1. Context values

   These are the values provided by the `pre-hook-command` output.
   
   ```json
   {
     "source": "context",
     "name": "parameter-name"
   }
   ``` 

2. HTTP Request Header values

    ```json
    {
      "source": "header",
      "name": "Header-Name"
    }
    ```

3. HTTP Query parameters

    ```json
    {
      "source": "url",
      "name": "parameter-name"
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