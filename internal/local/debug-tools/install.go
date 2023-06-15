package debug_tools

import (
	"context"
	"fmt"
	"path"
	"strings"

	"k8s.io/client-go/rest"

	"github.com/kebe7jun/miragedebug/api/app"
	"github.com/kebe7jun/miragedebug/internal/kube"
	langadaptors "github.com/kebe7jun/miragedebug/internal/lang-adaptors"
	"github.com/kebe7jun/miragedebug/internal/lang-adaptors/golang"
	"github.com/kebe7jun/miragedebug/internal/lang-adaptors/rust"
)

func InstallPodDebugTool(ctx context.Context, app_ *app.App, config *rest.Config, podName string) error {
	if app_.LocalConfig.DebugToolBuilder.Type == app.DebugToolType_REMOTE {
		out, outErr, err := kube.ExecutePodCmd(ctx,
			config,
			app_.RemoteRuntime.Namespace,
			podName,
			app_.RemoteRuntime.ContainerName,
			strings.Join(app_.LocalConfig.DebugToolBuilder.BuildCommands, "\n"),
			nil)
		if err != nil {
			return fmt.Errorf("failed to install remote debug tool: %s, %s, %v", out, outErr, err)
		}
		return nil
	}
	var langAdaptor langadaptors.LanguageAdaptor
	switch app_.ProgramType {
	case app.ProgramType_GO:
		langAdaptor = golang.NewGolangAdaptor()
	case app.ProgramType_RUST:
		langAdaptor = rust.NewRustAdaptor()
	default:
		return fmt.Errorf("not implemented")
	}
	f, err := langAdaptor.LocalDebugToolInstall(app_)
	if err != nil {
		return err
	}
	app_.LocalConfig.DebugToolBuilder.LocalDest = f
	err = kube.CopyLocalFileToPod(ctx,
		config,
		app_.RemoteRuntime.Namespace,
		podName,
		app_.RemoteRuntime.ContainerName,
		app_.LocalConfig.DebugToolBuilder.LocalDest,
		path.Base(app_.RemoteConfig.DebugToolPath),
		path.Dir(app_.RemoteConfig.DebugToolPath))
	if err != nil {
		return err
	}
	return nil
}
