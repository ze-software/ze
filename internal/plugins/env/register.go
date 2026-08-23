package env

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.MustRegisterRootHandler("env", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Environment variable inspection",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "list, get, registered",
	})
	// These three answer with DATA, so their answers go through the pipe layer
	// like any other command's. They printed a table and returned an exit code
	// before, which is why `ze cli -c "show env list | json"` answered
	// `unknown command`: YANG declared a wire method for each and no daemon
	// handler implemented one.
	registry.MustRegisterLocalData("show env list", dataList, registry.Meta{
		Description: "Every environment variable Ze reads, with its effective value.",
		Mode:        "offline",
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalData("show env get", dataGet, registry.Meta{
		Description: "One environment variable, by key.",
		Mode:        "offline",
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalData("show env registered", dataRegistered, registry.Meta{
		Description: "Every environment variable the code declares, without effective values.",
		Mode:        "offline",
	}, command.RenderLocalAnswer)

	// The rows are a list read against declared column names, so every row
	// operator applies and the published page can say so before the command
	// runs.
	command.RegisterShape([]string{"show env list", "show env get", "show env registered"}, command.ShapeTab)
	command.RegisterColumns([]string{"show env list", "show env get", "show env registered"},
		command.ColumnOrder{"key", "type", "default", "current", "description"},
	)
}
