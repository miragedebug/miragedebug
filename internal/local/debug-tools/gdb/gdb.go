package gdb

import (
	"path"

	"github.com/kebe7jun/miragedebug/api/app"
	"github.com/kebe7jun/miragedebug/config"
)

const gdbVersion = "v13.2"

func defaultGDBRoot() string {
	return path.Join(config.GetConfigRootPath(), "debug-tools", "gdb-"+gdbVersion)
}

func InstallingGDBServer(arch app.ArchType, cmds []string) (string, error) {
	// todo download gdbserver
	f := path.Join(defaultGDBRoot(), "gdbserver-"+app.ToSystemArch(arch))
	return f, nil
}
