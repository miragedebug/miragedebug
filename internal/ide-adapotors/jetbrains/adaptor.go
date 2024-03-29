package jetbrains

import (
	"fmt"
	"os"
	"path"

	"github.com/miragedebug/miragedebug/api/app"
	ideadapotors "github.com/miragedebug/miragedebug/internal/ide-adapotors"
)

const prepareScriptName = "Mirage - Prepare"

type jetbrainsAdaptor struct {
}

func NewJetbrainsAdaptor() ideadapotors.IDEAdaptor {
	return &jetbrainsAdaptor{}
}

func (j *jetbrainsAdaptor) initPreloadScript(name string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	configName := fmt.Sprintf("%s %s", prepareScriptName, name)
	runTmpl := `
<component name="ProjectRunConfigurationManager">
  <configuration default="false" name="%s" type="ShConfigurationType">
    <option name="SCRIPT_TEXT" value="%s debug %s" />
    <option name="INDEPENDENT_SCRIPT_PATH" value="true" />
    <option name="SCRIPT_PATH" value="" /> 
    <option name="SCRIPT_OPTIONS" value="" />
    <option name="INDEPENDENT_SCRIPT_WORKING_DIRECTORY" value="true" />
    <option name="SCRIPT_WORKING_DIRECTORY" value="$PROJECT_DIR$" />
    <option name="INDEPENDENT_INTERPRETER_PATH" value="true" />
    <option name="INTERPRETER_PATH" value="/bin/sh" />
    <option name="INTERPRETER_OPTIONS" value="" />
    <option name="EXECUTE_IN_TERMINAL" value="false" />
    <option name="EXECUTE_SCRIPT_FILE" value="false" />
    <envs />
    <method v="2" />
  </configuration>
</component>
`
	xml := fmt.Sprintf(runTmpl, configName, exe, name)
	f := path.Join(".run", fmt.Sprintf("%s.run.xml", configName))
	os.MkdirAll(path.Dir(f), 0755)
	if err := os.WriteFile(f, []byte(xml), 0644); err != nil {
		return err
	}
	return nil
}

func (j *jetbrainsAdaptor) initGolandRunRemoteConfig(name string, port int32) error {
	configName := fmt.Sprintf("Mirage - Remote Debug %s", name)
	runTmpl := `
<component name="ProjectRunConfigurationManager">
  <configuration default="false" name="%s" type="GoRemoteDebugConfigurationType" factoryName="Go Remote" port="%d">
    <option name="disconnectOption" value="STOP" />
    <method v="2">
      <option name="RunConfigurationTask" enabled="true" run_configuration_name="%s" run_configuration_type="ShConfigurationType" />
    </method>
  </configuration>
</component>
`
	xml := fmt.Sprintf(runTmpl, configName, port, fmt.Sprintf("%s %s", prepareScriptName, name))
	f := path.Join(".run", fmt.Sprintf("%s.run.xml", configName))
	os.MkdirAll(path.Dir(f), 0755)
	if err := os.WriteFile(f, []byte(xml), 0644); err != nil {
		return err
	}
	return nil
}

func (j *jetbrainsAdaptor) initCLionRunRemoteConfig(name string, port int32) error {
	configName := fmt.Sprintf("Mirage - Remote Debug %s", name)
	runTmpl := `
<component name="ProjectRunConfigurationManager">
  <configuration default="false" name="%s" type="CLion_Remote" version="1" remoteCommand="127.0.0.1:%d" symbolFile="" sysroot="">
    <debugger kind="GDB" isBundled="true" />
    <method v="2">
      <option name="RunConfigurationTask" enabled="true" run_configuration_name="%s" run_configuration_type="ShConfigurationType" />
    </method>
  </configuration>
</component>
`
	xml := fmt.Sprintf(runTmpl, configName, port, fmt.Sprintf("%s %s", prepareScriptName, name))
	f := path.Join(".run", fmt.Sprintf("%s.run.xml", configName))
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
	switch a.LocalConfig.IdeType {
	case app.IDEType_GOLAND:
		if err := j.initGolandRunRemoteConfig(a.Name, a.RemoteConfig.RemoteDebuggingPort); err != nil {
			return err
		}
	case app.IDEType_CLION:
		if err := j.initCLionRunRemoteConfig(a.Name, a.RemoteConfig.RemoteDebuggingPort); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported ide type: %s", a.LocalConfig.IdeType)
	}
	return nil
}
