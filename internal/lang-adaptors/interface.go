package langadaptors

import "github.com/kebe7jun/miragedebug/api/app"

type LanguageAdaptor interface {
	BuildCommand(a *app.App) (string, error)
	InstallLocalDebugTool(a *app.App) error
}
