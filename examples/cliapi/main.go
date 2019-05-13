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