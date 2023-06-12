package ideadapotors

import "github.com/kebe7jun/miragedebug/api/app"

type IDEAdaptor interface {
	// PrepareLaunch prepares the config for the IDE to launch the debugger
	PrepareLaunch(a *app.App) error
}
