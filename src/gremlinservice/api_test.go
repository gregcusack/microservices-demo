package main

import (
	"fmt"
	"gotest.tools/v3/assert"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestAPI(t *testing.T) {
	path := filepath.Join("data", "experiments", "1586318933")

	cmd := exec.Command(
		"sh",
		"mine.sh",
		filepath.Join(path, "traces.data"),
		filepath.Join(path, "traces.result"),
	)

	err := cmd.Start()
	assert.NilError(t, err)

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)

	time.AfterFunc(10*time.Second, func() {
		syscall.Kill(-pgid, 15)
	})

	err = cmd.Wait()
	assert.NilError(t, err)

	fmt.Println("Finished script")

	subgraphs, err := parseDags(path)
	assert.NilError(t, err)
	for _, g := range subgraphs {
		fmt.Println(g)
	}
}
