package jetbrains

import (
	"fmt"
	"os"
	"path"

	"github.com/kebe7jun/miragedebug/api/app"
	"github.com/kebe7jun/miragedebug/config"
	ideadapotors "github.com/kebe7jun/miragedebug/internal/ide-adapotors"
)

const (
	preloadScriptName = "mirage-debug-preload"
)

type jetbrainsAdaptor struct {
}

func NewJetbrainsAdaptor() ideadapotors.IDEAdaptor {
	return &jetbrainsAdaptor{}
}

func (j *jetbrainsAdaptor) initPreloadScript(name string) error {
	runTmpl := `
<component name="ProjectRunConfigurationManager">
  <configuration default="false" name="prepare-and-build-%s" type="ShConfigurationType">
    <option name="SCRIPT_TEXT" value="%s debug %s" />
    <option name="INDEPENDENT_SCRIPT_PATH" value="true" />
    <option name="SCRIPT_PATH" value="" /> 
    <option name="SCRIPT_OPTIONS" value="" />
    <option name="INDEPENDENT_SCRIPT_WORKING_DIRECTORY" value="true" />
    <option name="SCRIPT_WORKING_DIRECTORY" value="$PROJECT_DIR$" />
    <option name="INDEPENDENT_INTERPRETER_PATH" value="true" />
    <option name="INTERPRETER_PATH" value="/bin/sh" />
    <option name="INTERPRETER_OPTIONS" value="" />
    <option name="EXECUTE_IN_TERMINAL" value="true" />
    <option name="EXECUTE_SCRIPT_FILE" value="false" />
    <envs />
    <method v="2" />
  </configuration>
</component>
`
	xml := fmt.Sprintf(runTmpl, name, path.Join(config.GetConfigRootPath(), "bin", "mirage-debug"), name)
	f := path.Join(".run", fmt.Sprintf("mirage-debug-%s.run.xml", name))
	os.MkdirAll(path.Dir(f), 0755)
	if err := os.WriteFile(f, []byte(xml), 0644); err != nil {
		return err
	}
	return nil
}

func (j *jetbrainsAdaptor) initRunRemoteConfig(name string, port int32) error {
	runTmpl := `
<component name="ProjectRunConfigurationManager">
  <configuration default="false" name="Remote debug %s" type="GoRemoteDebugConfigurationType" factoryName="Go Remote" port="%d">
    <option name="disconnectOption" value="STOP" />
    <method v="2">
      <option name="RunConfigurationTask" enabled="true" run_configuration_name="prepare-and-build-%s" run_configuration_type="ShConfigurationType" />
    </method>
  </configuration>
</component>
`
	xml := fmt.Sprintf(runTmpl, name, port, name)
	f := path.Join(".run", fmt.Sprintf("mirage-debug-remote-debug-%s.run.xml", name))
	os.MkdirAll(path.Dir(f), 0755)
	if err := os.WriteFile(f, []byte(xml), 0644); err != nil {
		return err
	}
	return nil
}

func (j *jetbrainsAdaptor) PrepareLaunch(a *app.App) error {
	pwd, _ := os.Getwd()
	if pwd != a.LocalConfig.WorkingDir {
		return fmt.Errorf("you are not in the project root directory(%s)", a.LocalConfig.WorkingDir)
	}
	if err := j.initPreloadScript(a.Name); err != nil {
		return err
	}
	if err := j.initRunRemoteConfig(a.Name, a.RemoteConfig.RemoteDebuggingPort); err != nil {
		return err
	}
	return nil
}
