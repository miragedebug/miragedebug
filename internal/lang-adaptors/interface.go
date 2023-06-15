package langadaptors

import "github.com/kebe7jun/miragedebug/api/app"

type RemotePodShellExecutor func(commands []string) ([]byte, []byte, error)

type LanguageAdaptor interface {
	BuildCommand(a *app.App) (string, error)
	LocalDebugToolInstall(a *app.App) (string, error)
	DebugCommand(app_ *app.App) (string, error)
}
