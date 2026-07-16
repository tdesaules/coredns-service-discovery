package discovery

import (
	"context"
	"testing"
	"time"
)

func TestStaticSource_Name(t *testing.T) {
	s := &staticSource{}
	if s.Name() != "static" {
		t.Errorf("expected name 'static', got %q", s.Name())
	}
}

func TestStaticSource_ParseConfig_Basic(t *testing.T) {
	s := &staticSource{}
	c := testController(`
		instance a1b2c3 open-webui 10.88.0.5 8080
	}
	`)

	if err := s.ParseConfig(c); err != nil {
		t.Fatalf("ParseConfig error: %v", err)
	}

	if len(s.instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(s.instances))
	}

	si := s.instances[0]
	if si.service != "open-webui" {
		t.Errorf("expected service 'open-webui', got %q", si.service)
	}
	if si.namespace != "" {
		t.Errorf("expected empty namespace, got %q", si.namespace)
	}
	if si.instance.ID != "a1b2c3" {
		t.Errorf("expected ID 'a1b2c3', got %q", si.instance.ID)
	}
	if si.instance.Address != "10.88.0.5" {
		t.Errorf("expected address '10.88.0.5', got %q", si.instance.Address)
	}
	if si.instance.Port != 8080 {
		t.Errorf("expected port 8080, got %d", si.instance.Port)
	}
	if si.instance.Source != "static" {
		t.Errorf("expected source 'static', got %q", si.instance.Source)
	}
}

func TestStaticSource_ParseConfig_WithNamespace(t *testing.T) {
	s := &staticSource{}
	c := testController(`
		instance a1b2c3 open-webui 10.88.0.5 8080 prod
	}
	`)

	if err := s.ParseConfig(c); err != nil {
		t.Fatalf("ParseConfig error: %v", err)
	}

	if s.instances[0].namespace != "prod" {
		t.Errorf("expected namespace 'prod', got %q", s.instances[0].namespace)
	}
}

func TestStaticSource_ParseConfig_WithNamespaceAndProtocol(t *testing.T) {
	s := &staticSource{}
	c := testController(`
		instance a1b2c3 open-webui 10.88.0.5 8080 prod udp
	}
	`)

	if err := s.ParseConfig(c); err != nil {
		t.Fatalf("ParseConfig error: %v", err)
	}

	if s.instances[0].namespace != "prod" {
		t.Errorf("expected namespace 'prod', got %q", s.instances[0].namespace)
	}
	if s.instances[0].instance.Protocol != "udp" {
		t.Errorf("expected protocol 'udp', got %q", s.instances[0].instance.Protocol)
	}
}

func TestStaticSource_ParseConfig_MultipleInstances(t *testing.T) {
	s := &staticSource{}
	c := testController(`
		instance a1b2c3 open-webui 10.88.0.5 8080
		instance d4e5f6 open-webui 10.88.0.6 8080
		instance vm1 web-frontend 10.10.10.5 443
	}
	`)

	if err := s.ParseConfig(c); err != nil {
		t.Fatalf("ParseConfig error: %v", err)
	}

	if len(s.instances) != 3 {
		t.Fatalf("expected 3 instances, got %d", len(s.instances))
	}
}

func TestStaticSource_ParseConfig_TooFewArgs(t *testing.T) {
	s := &staticSource{}
	c := testController(`
		instance a1b2c3 open-webui 10.88.0.5
	}
	`)

	if err := s.ParseConfig(c); err == nil {
		t.Fatal("expected error for too few args")
	}
}

func TestStaticSource_ParseConfig_InvalidPort(t *testing.T) {
	s := &staticSource{}
	c := testController(`
		instance a1b2c3 open-webui 10.88.0.5 abc
	}
	`)

	if err := s.ParseConfig(c); err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestStaticSource_ParseConfig_UnknownDirective(t *testing.T) {
	s := &staticSource{}
	c := testController(`
		foobar a1b2c3
	}
	`)

	if err := s.ParseConfig(c); err == nil {
		t.Fatal("expected error for unknown directive")
	}
}

func TestStaticSource_ParseConfig_EmptyBlock(t *testing.T) {
	s := &staticSource{}
	c := testController(`
	}
	`)

	if err := s.ParseConfig(c); err != nil {
		t.Fatalf("ParseConfig error: %v", err)
	}

	if len(s.instances) != 0 {
		t.Fatalf("expected 0 instances, got %d", len(s.instances))
	}
}

func TestStaticSource_Run_PopulatesStore(t *testing.T) {
	s := &staticSource{
		instances: []staticInstance{
			{
				service:   "open-webui",
				namespace: "default",
				instance: &Instance{
					ID:      "a1b2c3",
					Address: "10.88.0.5",
					Port:    8080,
					Source:  "static",
				},
			},
			{
				service:   "web-frontend",
				namespace: "default",
				instance: &Instance{
					ID:      "vm1",
					Address: "10.10.10.5",
					Port:    443,
					Source:  "static",
				},
			},
		},
	}

	store := NewStore()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- s.Run(ctx, store)
	}()

	if !waitForInstances(store, "open-webui", "default", 1, time.Second) {
		t.Fatal("expected 1 open-webui instance in store")
	}

	inst := store.GetInstances("open-webui", "default")[0]
	if inst.Address != "10.88.0.5" {
		t.Errorf("expected address '10.88.0.5', got %q", inst.Address)
	}

	if !waitForInstances(store, "web-frontend", "default", 1, time.Second) {
		t.Fatal("expected 1 web-frontend instance in store")
	}

	inst = store.GetInstances("web-frontend", "default")[0]
	if inst.Port != 443 {
		t.Errorf("expected port 443, got %d", inst.Port)
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestStaticSource_Run_DeregistersOnExit(t *testing.T) {
	s := &staticSource{
		instances: []staticInstance{
			{
				service:   "open-webui",
				namespace: "default",
				instance: &Instance{
					ID:      "a1b2c3",
					Address: "10.88.0.5",
					Port:    8080,
					Source:  "static",
				},
			},
		},
	}

	store := NewStore()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- s.Run(ctx, store)
	}()

	if !waitForInstances(store, "open-webui", "default", 1, time.Second) {
		t.Fatal("expected 1 instance before cancel")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancel")
	}

	if len(store.GetInstances("open-webui", "default")) != 0 {
		t.Fatal("expected 0 instances after cancel (deregistered)")
	}
}

func TestStaticSource_Run_EmptyInstances(t *testing.T) {
	s := &staticSource{}
	store := NewStore()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- s.Run(ctx, store)
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestStaticSource_Integration_WithParseConfig(t *testing.T) {
	controller := testController(`
		discovery svc.desaules.in {
			ttl 30
			source static {
				instance a1b2c3 open-webui 10.88.0.5 8080
				instance d4e5f6 open-webui 10.88.0.6 8080
			}
		}
	`)

	h, sources, err := parseConfig(controller)
	if err != nil {
		t.Fatalf("parseConfig error: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	if sources[0].Name() != "static" {
		t.Errorf("expected source name 'static', got %q", sources[0].Name())
	}

	ss := sources[0].(*staticSource)
	if len(ss.instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(ss.instances))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ss.Run(ctx, h.Store)
	}()

	if !waitForInstances(h.Store, "open-webui", "default", 2, time.Second) {
		t.Fatalf("expected 2 instances in store, got %d", len(h.Store.GetInstances("open-webui", "default")))
	}
}

func waitForInstances(store *Store, service, namespace string, count int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(store.GetInstances(service, namespace)) == count {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func TestStaticSource_Run_RegisterError(t *testing.T) {
	s := &staticSource{
		instances: []staticInstance{
			{
				service:   "test",
				namespace: "default",
				instance:  &Instance{ID: "", Address: "10.0.0.1", Port: 8080, Source: "static"},
			},
		},
	}
	store := NewStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := s.Run(ctx, store)
	if err == nil {
		t.Fatal("expected error for invalid instance")
	}
}
