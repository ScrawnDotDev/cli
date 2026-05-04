package cmd

type Context struct {
	Flags map[string]string
}

func (c *Context) GetFlag(name string) string {
	return c.Flags[name]
}

func (c *Context) SetFlag(name, value string) {
	if c.Flags == nil {
		c.Flags = make(map[string]string)
	}
	c.Flags[name] = value
}