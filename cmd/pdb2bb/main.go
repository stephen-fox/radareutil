package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/stephen-fox/radareutil"
)

func main() {
	help := flag.Bool("h", false, "Displays this help page")

	flag.Parse()

	if *help {
		os.Stderr.WriteString(`pdb2bb

Converts the output of radare2's 'pdb' provided via stdin as a pretty-formatted
basic block.

options:
`)
		flag.PrintDefaults()
		os.Exit(1)
	}

	result, err := radareutil.PdbToBasicBlockText(os.Stdin)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(result)
}
