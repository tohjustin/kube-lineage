package main_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	lineage "github.com/tohjustin/kube-lineage/cmd/kube-lineage"
	"github.com/tohjustin/kube-lineage/internal/version"
)

func runCmd(args ...string) (string, error) {
	buf := bytes.NewBufferString("")
	streams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	cmd := lineage.New(streams)
	cmd.SetOut(buf)

	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return "", err
	}
	out, err := io.ReadAll(buf)
	if err != nil {
		return "", err
	}

	return string(out), nil
}

func TestCommandWithVersionFlag(t *testing.T) {
	t.Parallel()

	output, err := runCmd("--version")
	if err != nil {
		t.Fatalf("failed to run command: %v", err)
	}

	expected := fmt.Sprintf("%#v\n", version.Get())
	if output != expected {
		t.Fatalf("expected \"%s\" got \"%s\"", expected, output)
	}
}
