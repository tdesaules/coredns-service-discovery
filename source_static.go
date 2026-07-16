package discovery

import (
	"context"
	"strconv"

	"github.com/coredns/caddy"
)

func init() {
	RegisterSource("static", func() Source { return &staticSource{} })
}

type staticInstance struct {
	service   string
	namespace string
	instance  *Instance
}

type staticSource struct {
	instances []staticInstance
}

func (s *staticSource) Name() string { return "static" }

func (s *staticSource) ParseConfig(c *caddy.Controller) error {
	s.instances = nil

	for c.Next() {
		if c.Val() == "}" {
			break
		}

		switch c.Val() {
		case "instance":
			args := c.RemainingArgs()
			if len(args) < 4 {
				return c.Errf("instance requires at least 4 args: <id> <service> <address> <port> [<namespace>] [<protocol>]")
			}

			port, err := strconv.Atoi(args[3])
			if err != nil {
				return c.Errf("invalid port %q: %v", args[3], err)
			}

			inst := &Instance{
				ID:      args[0],
				Address: args[2],
				Port:    port,
				Source:  "static",
			}

			si := staticInstance{
				service:  args[1],
				instance: inst,
			}

			if len(args) >= 5 {
				si.namespace = args[4]
			}

			if len(args) >= 6 {
				inst.Protocol = args[5]
			}

			s.instances = append(s.instances, si)

		default:
			return c.Errf("unknown directive %q in static source", c.Val())
		}
	}

	return nil
}

func (s *staticSource) Run(ctx context.Context, store *Store) error {
	defer store.DeregisterBySource("static")

	for _, si := range s.instances {
		if err := store.Register(si.service, si.namespace, si.instance); err != nil {
			return err
		}
	}

	<-ctx.Done()

	return nil
}
