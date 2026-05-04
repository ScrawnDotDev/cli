package cmd

type Command interface {
	Name() string
	Description() string
	Run(ctx *Context, args []string) error
}

var registry = make(map[string]func() Command)

func Register(name string, factory func() Command) {
	registry[name] = factory
}

func Get(name string) (Command, bool) {
	f, ok := registry[name]
	if !ok {
		return nil, false
	}
	return f(), true
}

func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}