# radareutil
A Go library for working with the radare2 debugging utility.

## API
This library currently provides very rudimentary support for radare2's CLI
and HTTP APIs. This is achieved using a basic interface called `Api`.
The underlying radare2 process is managed and interacted with using this
interface. It is **strongly recommended** that you use the CLI API
implementation to drive radare2. The HTTP API is not nearly as secure as the
CLI API. You should only use the HTTP API if you know what you are doing (at
which point you should ask yourself where it all went wrong).

#### Instantiating a new instance of radare2 and an API
As noted before, you should **really** use the CLI API to drive radare2. This
is done using the `NewCliApi()` function:
```go
package main

import (
	"fmt"
	"log"

	"github.com/stephen-fox/radareutil"
)

func main() {
	cliApi, err := radareutil.NewCliApi(&radareutil.Radare2Config{
		// The executable path does not need to be fully qualified.
		ExecutablePath: "radare2",
	})
	if err != nil {
		log.Fatalf("failed to create a CLI API - %s", err.Error())
	}
	
	err = cliApi.Start()
	if err != nil {
		log.Fatalf("failed to start CLI API - %s", err.Error())
	}
	defer cliApi.Kill()
	
	output, err := cliApi.Execute("?")
	if err != nil {
		log.Fatalf("failed to execute API command - %s", err.Error())
	}

	fmt.Println(output)
}
```
