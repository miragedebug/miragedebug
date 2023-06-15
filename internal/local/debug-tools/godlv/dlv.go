package godlv

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/miragedebug/miragedebug/api/app"
	"github.com/miragedebug/miragedebug/config"
	"github.com/miragedebug/miragedebug/pkg/log"
	"github.com/miragedebug/miragedebug/pkg/shell"
)

const (
	dlvVersion = "v1.20.1"
)

func defaultDlvRoot() string {
	return path.Join(config.GetConfigRootPath(), "debug-tools", "dlv-"+dlvVersion)
}

// InitOrLoadDLV initializes or loads the dlv binary for the given architecture
func InitOrLoadDLV(arch app.ArchType, cmds []string) (string, error) {
	root := defaultDlvRoot()
	f := path.Join(root, "dlv-"+app.ToSystemArch(arch))
	if _, err := os.Stat(f); err == nil {
		return f, nil
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	var commands []string
	if len(cmds) > 0 {
		commands = cmds
	} else {
		t, err := os.MkdirTemp("", "dlv-install")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(t)
		commands = []string{
			fmt.Sprintf("git clone -b %s --depth=1 https://github.com/go-delve/delve %s", dlvVersion, t),
			fmt.Sprintf("pushd %s", t),
			fmt.Sprintf("GOOS=linux GOARCH=%s go build -o ./dlv-%s ./cmd/dlv", app.ToSystemArch(arch), app.ToSystemArch(arch)),
		}
		defer os.Rename(path.Join(t, "dlv-"+app.ToSystemArch(arch)), f)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()
	if out, errOut, err := shell.ExecuteCommands(ctx, commands); err != nil {
		log.Errorf("Failed to install dlv: %v, stdout: %s, stderr: %s", err, out, errOut)
		return "", err
	}
	return f, nil
}
