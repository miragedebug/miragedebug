package golang

import (
	"fmt"
	"path"

	"github.com/miragedebug/miragedebug/api/app"
	langadaptors "github.com/miragedebug/miragedebug/internal/lang-adaptors"
	debugtools "github.com/miragedebug/miragedebug/internal/local/debug-tools/godlv"
)

type golang struct{}

func NewGolangAdaptor() langadaptors.LanguageAdaptor {
	return &golang{}
}

func (g *golang) BuildCommand(a *app.App) (string, error) {
	if a.ProgramType != app.ProgramType_GO {
		return "", fmt.Errorf("program type is not go")
	}
	if a.LocalConfig.CustomBuildCommand != "" {
		return a.LocalConfig.CustomBuildCommand, nil
	}
	return fmt.Sprintf("GOOS=linux GOARCH=%s go build -o %s %s", app.ToSystemArch(a.RemoteRuntime.TargetArch), a.LocalConfig.BuildOutput, a.LocalConfig.AppEntryPath), nil
}

func (g *golang) DebugCommand(app_ *app.App) (string, error) {
	if app_.ProgramType != app.ProgramType_GO {
		return "", fmt.Errorf("program type is not go")
	}
	return fmt.Sprintf("%s --listen=:%d --headless=true --api-version=2 --accept-multiclient --check-go-version=false exec -- %s %s",
		app_.RemoteConfig.DebugToolPath,
		app_.RemoteConfig.RemoteDebuggingPort,
		path.Join(app_.RemoteConfig.RemoteAppLocation, path.Base(app_.LocalConfig.BuildOutput)),
		app_.LocalConfig.AppArgs,
	), nil
}

func (g *golang) LocalDebugToolInstall(a *app.App) (string, error) {
	return debugtools.InitOrLoadDLV(a.RemoteRuntime.TargetArch, a.LocalConfig.DebugToolBuilder.BuildCommands)
}
