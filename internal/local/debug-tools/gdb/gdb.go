package gdb

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"

	"github.com/miragedebug/miragedebug/api/app"
	"github.com/miragedebug/miragedebug/config"
)

const gdbVersion = "v13.2"

func defaultGDBRoot() string {
	return path.Join(config.GetConfigRootPath(), "debug-tools", "gdb-"+gdbVersion)
}

func downloadFile(url, filename string) error {
	// download file from url
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	os.MkdirAll(path.Dir(filename), 0755)
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func InstallingGDBServer(arch app.ArchType, cmds []string) (string, error) {
	f := path.Join(defaultGDBRoot(), "gdbserver-"+app.ToSystemArch(arch))
	if _, err := os.Stat(f); err == nil {
		return f, nil
	}
	switch arch {
	case app.ArchType_AMD64:
		if err := downloadFile("https://github.com/miragedebug/gdb-static/raw/main/gdbserver-v13.2-amd64", f); err != nil {
			return "", err
		}
	case app.ArchType_ARM64:
		return "", fmt.Errorf("arm64 gdbserver not supported yet")
	}
	return f, nil
}
