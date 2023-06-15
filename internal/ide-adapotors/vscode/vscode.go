package vscode

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/samber/lo"
	"muzzammil.xyz/jsonc"

	"github.com/kebe7jun/miragedebug/api/app"
	"github.com/kebe7jun/miragedebug/config"
	ideadapotors "github.com/kebe7jun/miragedebug/internal/ide-adapotors"
)

type vscodeAdaptor struct {
}

func NewVSCodeAdaptor() ideadapotors.IDEAdaptor {
	return &vscodeAdaptor{}
}

type taskConfig struct {
	Version string                   `json:"version"`
	Tasks   []map[string]interface{} `json:"tasks"`
}

type launchConfig struct {
	Version string                   `json:"version"`
	Configs []map[string]interface{} `json:"configurations"`
}

func (j *vscodeAdaptor) initPreloadScript(name string) ([]byte, error) {
	prepareTask := map[string]interface{}{
		"type":    "shell",
		"label":   fmt.Sprintf("prepare-and-build-%s", name),
		"command": path.Join(config.GetConfigRootPath(), "bin", "mirage-debug"),
		"args": []string{
			"debug",
			name,
		},
	}
	bs, _ := json.Marshal(prepareTask)
	taskFile := path.Join(".vscode", "tasks.json")
	os.MkdirAll(path.Dir(taskFile), 0755)
	tc := taskConfig{}
	if _, err := os.Stat(taskFile); os.IsNotExist(err) {
		tc.Version = "2.0.0"
		tc.Tasks = []map[string]interface{}{prepareTask}
	} else {
		j, _ := os.ReadFile(taskFile)
		jc := jsonc.ToJSON([]byte(j))
		if err := json.Unmarshal(jc, &tc); err != nil {
			return bs, err
		}
		tc.Tasks = lo.Filter(tc.Tasks, func(item map[string]interface{}, index int) bool {
			return item["label"] != prepareTask["label"]
		})
		tc.Tasks = append(tc.Tasks, prepareTask)
	}
	tcbs, err := json.MarshalIndent(tc, "", "  ")
	if err != nil {
		return bs, err
	}
	return bs, os.WriteFile(taskFile, tcbs, 0644)
}

func (j *vscodeAdaptor) initRunRemoteConfig(programType app.ProgramType, name string, port int32, buildOutput string, gdbpath string) ([]byte, error) {
	var launchTask map[string]interface{}
	label := fmt.Sprintf("Remote debug %s", name)
	switch programType {
	case app.ProgramType_GO:
		launchTask = map[string]interface{}{
			"name":          label,
			"type":          "go",
			"request":       "attach",
			"mode":          "remote",
			"remotePath":    "${workspaceFolder}",
			"port":          port,
			"host":          "127.0.0.1",
			"preLaunchTask": fmt.Sprintf("prepare-and-build-%s", name),
		}
	case app.ProgramType_RUST:
		launchTask = map[string]interface{}{
			"type":       "gdb",
			"request":    "attach",
			"name":       label,
			"executable": buildOutput,
			"target":     fmt.Sprintf("127.0.0.1:%d", port),
			"remote":     true,
			"cwd":        "${workspaceRoot}",
			"gdbpath": func() string {
				if gdbpath == "" {
					return "/usr/bin/gdb"
				}
				return gdbpath
			}(),
			"preLaunchTask": fmt.Sprintf("prepare-and-build-%s", name),
		}
	}
	bs, _ := json.Marshal(launchTask)
	taskFile := path.Join(".vscode", "launch.json")
	os.MkdirAll(path.Dir(taskFile), 0755)
	tc := launchConfig{}
	if _, err := os.Stat(taskFile); os.IsNotExist(err) {
		tc.Version = "2.0.0"
		tc.Configs = []map[string]interface{}{launchTask}
	} else {
		j, _ := os.ReadFile(taskFile)
		jc := jsonc.ToJSON([]byte(j))
		if err := json.Unmarshal(jc, &tc); err != nil {
			return bs, err
		}
		tc.Configs = lo.Filter(tc.Configs, func(item map[string]interface{}, index int) bool {
			return item["name"] != launchTask["name"]
		})
		tc.Configs = append(tc.Configs, launchTask)
	}
	tcbs, err := json.MarshalIndent(tc, "", "  ")
	if err != nil {
		return bs, err
	}
	return bs, os.WriteFile(taskFile, tcbs, 0644)
}

func (j *vscodeAdaptor) PrepareLaunch(a *app.App) error {
	pwd, _ := os.Getwd()
	if pwd != a.LocalConfig.WorkingDir {
		return fmt.Errorf("you are not in the project root directory(%s)", a.LocalConfig.WorkingDir)
	}
	if _, err := j.initPreloadScript(a.Name); err != nil {
		return err
	}
	_, err := j.initRunRemoteConfig(a.ProgramType, a.Name, a.RemoteConfig.RemoteDebuggingPort, a.LocalConfig.BuildOutput, a.LocalConfig.Metadata["gdbpath"])
	return err
}
