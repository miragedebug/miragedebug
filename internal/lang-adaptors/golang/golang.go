package golang

import (
	"fmt"

	"github.com/kebe7jun/miragedebug/api/app"
	debugtools "github.com/kebe7jun/miragedebug/internal/local/debug-tools/godlv"
)

type golang struct{}

func (g *golang) BuildCommand(a *app.App) (string, error) {
	if a.ProgramType != app.ProgramType_GO {
		return "", fmt.Errorf("program type is not go")
	}
	if a.LocalConfig.CustomBuildCommand != "" {
		return a.LocalConfig.CustomBuildCommand, nil
	}
	return fmt.Sprintf("GOOS=linux GOARCH=%s go build -o %s %s", a.RemoteRuntime.TargetArch, a.LocalConfig.BuildOutput, a.LocalConfig.AppEntryPath), nil
}

func (g *golang) InstallLocalDebugTool(a *app.App) error {
	if a.LocalConfig.DebugToolBuilder.Type != app.DebugToolType_LOCAL {
		return fmt.Errorf("debug tool type is not local")
	}
	f, err := debugtools.InitOrLoadDLV(a.RemoteRuntime.TargetArch, a.LocalConfig.DebugToolBuilder.BuildCommands)
	if err != nil {
		return err
	}
	a.LocalConfig.DebugToolBuilder.LocalDest = f
	return nil
}
