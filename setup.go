package discovery

import (
	"context"
	"fmt"
	"sync"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
)

var log = clog.NewWithPlugin("discovery")

func init() {
	plugin.Register("discovery", setup)
}

func setup(c *caddy.Controller) error {
	h, sources, err := parseConfig(c)
	if err != nil {
		return plugin.Error("discovery", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	c.OnStartup(func() error {
		for _, src := range sources {
			wg.Add(1)
			go func(s Source) {
				defer wg.Done()
				if err := s.Run(ctx, h.Store); err != nil {
					log.Errorf("source %s: %v", s.Name(), err)
				}
			}(src)
		}
		return nil
	})

	c.OnShutdown(func() error {
		cancel()
		wg.Wait()
		return nil
	})

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		h.Next = next
		return h
	})

	return nil
}

func parseConfig(c *caddy.Controller) (*DiscoveryHandler, []Source, error) {
	c.Next()

	args := c.RemainingArgs()
	if len(args) == 0 {
		return nil, nil, c.Err("zone is required")
	}
	zone := args[0]
	if zone == "" {
		return nil, nil, c.Err("zone is required")
	}

	h := &DiscoveryHandler{
		Store: NewStore(),
		Zone:  plugin.Name(zone).Normalize(),
		TTL:   30,
	}

	var sources []Source

	for c.NextBlock() {
		switch c.Val() {
		case "ttl":
			if !c.NextArg() {
				return nil, nil, c.ArgErr()
			}
			ttl, err := parseTTL(c.Val())
			if err != nil {
				return nil, nil, err
			}
			h.TTL = ttl

		case "fallthrough":
			// fallthrough is implicit — queries that don't match go to Next

		case "source":
			if !c.NextArg() {
				return nil, nil, c.ArgErr()
			}
			srcName := c.Val()
			factory, ok := GetSource(srcName)
			if !ok {
				registered := RegisteredSources()
				return nil, nil, c.Errf("unknown source %q, registered sources: %v", srcName, registered)
			}
			src := factory()

			// Check for sub-block opening brace on same line
			if c.NextArg() {
				if c.Val() != "{" {
					return nil, nil, c.Errf("expected '{' after source name, got %q", c.Val())
				}
				// Source parses its own sub-block using c.Next() until "}"
				if err := src.ParseConfig(c); err != nil {
					return nil, nil, fmt.Errorf("source %s: %w", srcName, err)
				}
			}

			sources = append(sources, src)

		default:
			return nil, nil, c.Errf("unknown directive %q", c.Val())
		}
	}

	return h, sources, nil
}

func parseTTL(s string) (uint32, error) {
	var ttl uint32
	_, err := fmt.Sscanf(s, "%d", &ttl)
	if err != nil {
		return 0, fmt.Errorf("invalid TTL %q: %w", s, err)
	}
	if ttl > 3600 {
		return 0, fmt.Errorf("TTL %d exceeds maximum of 3600", ttl)
	}
	return ttl, nil
}
