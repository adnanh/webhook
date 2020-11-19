# Pre-hook command
To the STDIN of the pre-hook command, webhook will pass a JSON string representation of an object with the following properties:
 * `hookID` - ID of the hook that got matched
 * `method` - HTTP(s) method used by the client (i.e. GET, POST, etc...)
 * `URI` - URI which client requested
 * `host` - value of the `Host` header sent by the client
 * `remoteAddr` - client's IP address and port in the `IP:PORT` format
 * `query` - object with query parameters and their respective values
 * `headers` - object with headers and their respective values
 * `base64EncodedBody` - base64 encoded request body

_Please note!_ Output of this command __MUST__ be valid JSON string which will be parsed by the webhook and accessible using the `pre-hook` as source when referencing values. 

__Important! Any errors encountered while trying to execute the pre-hook command will prevent the hook from triggering!__

# Examples

_Please note:_ Following examples use shell scripts as pre-hook commands, but it is possible to use ruby, python, or anything else you like, as long as it outputs a valid JSON string as the result.

_Make sure you have the `jq` command available, as we're using it to parse the JSON in the pre-hook script._

## Getting the IP address of the requester
<details>
    <summary>script.sh</summary>
    
    ```sh
    #!/bin/bash

    ip=$1
    
    echo $ip >> ips.txt
    ```
</details>
<details>
    <summary>prehook.sh</summary>
    
    ```sh
    #!/bin/bash
    
    context=$(cat)
    ip=`echo $context | jq -r '.remoteAddr' | cut -d ':' -f 1`
    
    echo "{\"ip\": \"$ip\"}"
    ```
</details>

<details>
    <summary>hooks.json</summary>
    
    ```json
    [
        {
            "id": "log-ip",
            "pre-hook-command": "/home/example/prehook.sh",
            "execute-command": "/home/example/script.sh",
            "pass-arguments-to-command": [
                { "source": "pre-hook", "name": "ip" }
            ]
        }
    ]
    ```
</details>