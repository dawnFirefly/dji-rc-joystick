package main

import (
	"fmt"
	"os"
)

// // // // // // // // // //

func main() {
	if len(os.Args) != 4 {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: language <source-directory> <output-file> <package-name>")
		os.Exit(2)
	}

	if err := generate(os.Args[1], os.Args[2], os.Args[3]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Language generation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\tGenerate: %s\n", os.Args[2])
}
