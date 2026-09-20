// Command cdclint lints the contract between a source database schema, a
// change-data-capture connector configuration and the sink schema that
// reads the captured stream.
//
// The three are text files in a repository, written by different people at
// different times, and nothing reads them together. A column present in the
// source and read by the sink but absent from the connector's include list is
// dropped before it reaches the stream, silently: every message is
// well-formed and the sink fills the column with its type's default on every
// row. cdclint fails the pull request instead.
package main

import (
	"fmt"
	"os"
)

// version is set by the release build with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		fmt.Println("cdclint", version)
		return
	}
	fmt.Fprintln(os.Stderr, "cdclint: no rules are implemented yet; see README.md for the plan")
	os.Exit(2)
}
