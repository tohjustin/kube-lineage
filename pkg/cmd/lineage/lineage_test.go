package lineage_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tohjustin/kubectl-lineage/internal/version"
	"github.com/tohjustin/kubectl-lineage/pkg/cmd/lineage"
)

func runCmd(args ...string) (string, error) {
	b := bytes.NewBufferString("")
	cmd := lineage.New(genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	})
	cmd.SetOut(b)

	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return "", err
	}
	out, err := io.ReadAll(b)
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
