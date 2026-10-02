// c1apprun: thin launcher for C1ancher local-apps entries.
// The local-apps launcher already holds the external-app lease and passes
// inherited fds; this binary only execs the command stored next to itself:
// basename(argv[0]) without the "-run" suffix selects <id>.cmd, e.g.
// /storage/c1/local-apps/bin/zork-run reads zork.cmd next to it.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func main() {
	id := strings.TrimSuffix(filepath.Base(os.Args[0]), "-run")
	cmdFile := filepath.Join(filepath.Dir(os.Args[0]), id+".cmd")
	data, err := os.ReadFile(cmdFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "c1apprun: read %s: %v\n", cmdFile, err)
		os.Exit(2)
	}
	cmd := strings.TrimSpace(string(data))
	if cmd == "" {
		fmt.Fprintf(os.Stderr, "c1apprun: empty command in %s\n", cmdFile)
		os.Exit(2)
	}
	argv := []string{"/bin/sh", "-c", cmd}
	if err := syscall.Exec("/bin/sh", argv, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "c1apprun: exec: %v\n", err)
		os.Exit(1)
	}
}
