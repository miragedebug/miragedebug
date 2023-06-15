package rust

import (
	"fmt"
	"path"

	restclient "k8s.io/client-go/rest"

	"github.com/miragedebug/miragedebug/api/app"
	langadaptors "github.com/miragedebug/miragedebug/internal/lang-adaptors"
	"github.com/miragedebug/miragedebug/internal/local/debug-tools/gdb"
)

type rust struct {
	config *restclient.Config
}

func NewRustAdaptor() langadaptors.LanguageAdaptor {
	return &rust{}
}

func (r *rust) DebugCommand(app_ *app.App) (string, error) {
	if app_.ProgramType != app.ProgramType_RUST {
		return "", fmt.Errorf("program type is not rust")
	}
	return fmt.Sprintf("%s *:%d %s %s",
		app_.RemoteConfig.DebugToolPath,
		app_.RemoteConfig.RemoteDebuggingPort,
		path.Join(app_.RemoteConfig.RemoteAppLocation, path.Base(app_.LocalConfig.BuildOutput)),
		app_.LocalConfig.AppArgs,
	), nil
}

func (r *rust) BuildCommand(a *app.App) (string, error) {
	if a.ProgramType != app.ProgramType_RUST {
		return "", fmt.Errorf("program type is not rust")
	}
	if a.LocalConfig.CustomBuildCommand != "" {
		return a.LocalConfig.CustomBuildCommand, nil
	}
	arch := func() string {
		switch a.RemoteRuntime.TargetArch {
		case app.ArchType_AMD64:
			return "x86_64"
		case app.ArchType_ARM64:
			return "aarch64"
		default:
			return ""
		}
	}()
	return fmt.Sprintf("cargo build --target %s-unknown-linux-gnu", arch), nil
}

func (r *rust) LocalDebugToolInstall(a *app.App) (string, error) {
	return gdb.InstallingGDBServer(a.RemoteRuntime.TargetArch, a.LocalConfig.DebugToolBuilder.BuildCommands)
}
