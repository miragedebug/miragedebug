package shell

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ExecuteCommands execute shell commands one by one,
// If one command failed, the rest commands will not be executed.
// Return the stdout and stderr of the all commands.
func ExecuteCommands(ctx context.Context, commands []string) ([]byte, []byte, error) {
	shell := fmt.Sprintf("set -e\n%s", strings.Join(commands, "\n"))
	cmd := exec.CommandContext(ctx, "bash", "-c", shell)
	out := bytes.NewBuffer(nil)
	errOut := bytes.NewBuffer(nil)
	cmd.Stdout = out
	cmd.Stderr = errOut
	err := cmd.Run()
	return out.Bytes(), errOut.Bytes(), err
}
