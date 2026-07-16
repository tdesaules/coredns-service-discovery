package discovery

import (
	"context"
	"fmt"
	"testing"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/plugin/test"
)

func TestSetup_BasicConfig(t *testing.T) {
	controller := testController(`
		discovery svc.desaules.in {
			ttl 30
		}
	`)

	h, sources, err := parseConfig(controller)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	if h.Zone != "svc.desaules.in." {
		t.Errorf("expected zone 'svc.desaules.in', got %q", h.Zone)
	}
	if h.TTL != 30 {
		t.Errorf("expected TTL 30, got %d", h.TTL)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(sources))
	}
}

func TestSetup_DefaultTTL(t *testing.T) {
	controller := testController(`
		discovery svc.desaules.in {
		}
	`)

	h, _, err := parseConfig(controller)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	if h.TTL != 30 {
		t.Errorf("expected default TTL 30, got %d", h.TTL)
	}
}

func TestSetup_WithSource(t *testing.T) {
	RegisterSource("mock", func() Source { return &mockSource{} })
	defer func() { sourceRegistryMu.Lock(); delete(sourceRegistry, "mock"); sourceRegistryMu.Unlock() }()

	controller := testController(`
		discovery svc.desaules.in {
			ttl 60
			source mock {
				socket /tmp/mock.sock
			}
		}
	`)

	h, sources, err := parseConfig(controller)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	if h.Zone != "svc.desaules.in." {
		t.Errorf("expected zone 'svc.desaules.in', got %q", h.Zone)
	}
	if h.TTL != 60 {
		t.Errorf("expected TTL 60, got %d", h.TTL)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].Name() != "mock" {
		t.Errorf("expected source name 'mock', got %q", sources[0].Name())
	}
}

func TestSetup_UnknownSource(t *testing.T) {
	controller := testController(`
		discovery svc.desaules.in {
			source nonexistent {
			}
		}
	`)

	_, _, err := parseConfig(controller)
	if err == nil {
		t.Fatal("expected error for unknown source")
	}
}

func TestSetup_MissingZone(t *testing.T) {
	controller := testController(`
		discovery {
		}
	`)

	_, _, err := parseConfig(controller)
	if err == nil {
		t.Fatal("expected error for missing zone")
	}
}

func TestSetup_InvalidTTL(t *testing.T) {
	controller := testController(`
		discovery svc.desaules.in {
			ttl abc
		}
	`)

	_, _, err := parseConfig(controller)
	if err == nil {
		t.Fatal("expected error for invalid TTL")
	}
}

func TestSetup_TTLTooHigh(t *testing.T) {
	controller := testController(`
		discovery svc.desaules.in {
			ttl 9999
		}
	`)

	_, _, err := parseConfig(controller)
	if err == nil {
		t.Fatal("expected error for TTL > 3600")
	}
}

func TestSetup_UnknownDirective(t *testing.T) {
	controller := testController(`
		discovery svc.desaules.in {
			foobar bar
		}
	`)

	_, _, err := parseConfig(controller)
	if err == nil {
		t.Fatal("expected error for unknown directive")
	}
}

func TestSetup_MultipleSources(t *testing.T) {
	RegisterSource("mock1", func() Source { return &mockSource{name: "mock1"} })
	RegisterSource("mock2", func() Source { return &mockSource{name: "mock2"} })
	defer func() {
		sourceRegistryMu.Lock()
		delete(sourceRegistry, "mock1")
		delete(sourceRegistry, "mock2")
		sourceRegistryMu.Unlock()
	}()

	controller := testController(`
		discovery svc.desaules.in {
			source mock1 {
			}
			source mock2 {
			}
		}
	`)

	_, sources, err := parseConfig(controller)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
}

func TestSetup_FunctionSuccess(t *testing.T) {
	c := testController(`discovery svc.desaules.in { ttl 30 }`)
	if err := setup(c); err != nil {
		t.Fatalf("setup() returned error: %v", err)
	}
}

func TestSetup_FunctionError(t *testing.T) {
	c := testController(`discovery { }`)
	if err := setup(c); err == nil {
		t.Fatal("expected error for missing zone")
	}
}

func TestSetup_FunctionWithSource(t *testing.T) {
	RegisterSource("mock", func() Source { return &mockSource{} })
	defer func() { sourceRegistryMu.Lock(); delete(sourceRegistry, "mock"); sourceRegistryMu.Unlock() }()

	c := testController(`discovery svc.desaules.in { source mock { } }`)
	if err := setup(c); err != nil {
		t.Fatalf("setup() returned error: %v", err)
	}
}

func TestSetup_FallthroughDirective(t *testing.T) {
	c := testController(`discovery svc.desaules.in { fallthrough }`)
	_, _, err := parseConfig(c)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
}

func TestSetup_EmptyZoneString(t *testing.T) {
	c := testController(`discovery "" { }`)
	_, _, err := parseConfig(c)
	if err == nil {
		t.Fatal("expected error for empty zone string")
	}
}

func TestSetup_TTLNoValue(t *testing.T) {
	c := testController(`
		discovery svc.desaules.in {
			ttl
		}
	`)
	_, _, err := parseConfig(c)
	if err == nil {
		t.Fatal("expected error for TTL with no value")
	}
}

func TestSetup_SourceNoName(t *testing.T) {
	c := testController(`
		discovery svc.desaules.in {
			source
		}
	`)
	_, _, err := parseConfig(c)
	if err == nil {
		t.Fatal("expected error for source with no name")
	}
}

func TestSetup_SourceMissingBrace(t *testing.T) {
	RegisterSource("mock", func() Source { return &mockSource{} })
	defer func() { sourceRegistryMu.Lock(); delete(sourceRegistry, "mock"); sourceRegistryMu.Unlock() }()

	c := testController(`discovery svc.desaules.in { source mock notabrace }`)
	_, _, err := parseConfig(c)
	if err == nil {
		t.Fatal("expected error for missing brace after source name")
	}
}

func TestSetup_SourceParseConfigError(t *testing.T) {
	RegisterSource("errsrc", func() Source { return &errorSource{} })
	defer func() { sourceRegistryMu.Lock(); delete(sourceRegistry, "errsrc"); sourceRegistryMu.Unlock() }()

	c := testController(`discovery svc.desaules.in { source errsrc { } }`)
	_, _, err := parseConfig(c)
	if err == nil {
		t.Fatal("expected error from source ParseConfig")
	}
}

func TestSetup_SourceWithoutSubBlock(t *testing.T) {
	RegisterSource("mock", func() Source { return &mockSource{} })
	defer func() { sourceRegistryMu.Lock(); delete(sourceRegistry, "mock"); sourceRegistryMu.Unlock() }()

	c := testController(`
		discovery svc.desaules.in {
			source mock
		}
	`)
	_, sources, err := parseConfig(c)
	if err != nil {
		t.Fatalf("parseConfig error: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
}

func testController(input string) *caddy.Controller {
	return caddy.NewTestController("dns", input)
}

type mockSource struct {
	name string
}

func (m *mockSource) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockSource) ParseConfig(c *caddy.Controller) error {
	for c.Next() {
		if c.Val() == "}" {
			break
		}
	}
	return nil
}

func (m *mockSource) Run(ctx context.Context, store *Store) error {
	<-ctx.Done()
	return nil
}

type errorSource struct{}

func (e *errorSource) Name() string { return "errsrc" }

func (e *errorSource) ParseConfig(c *caddy.Controller) error {
	return fmt.Errorf("parse error")
}

func (e *errorSource) Run(ctx context.Context, store *Store) error {
	return nil
}

var _ = test.ResponseWriter{}
