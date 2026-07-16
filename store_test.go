package discovery

import (
	"fmt"
	"sync"
	"testing"
)

func TestStore_Register(t *testing.T) {
	tests := []struct {
		name      string
		svcName   string
		namespace string
		instance  *Instance
		wantErr   bool
	}{
		{
			name:      "valid instance",
			svcName:   "myapp",
			namespace: "default",
			instance:  &Instance{ID: "a1b2c3", Address: "10.0.0.1", Port: 8080, Source: "podman"},
			wantErr:   false,
		},
		{
			name:      "empty namespace defaults to default",
			svcName:   "myapp",
			namespace: "",
			instance:  &Instance{ID: "a1b2c3", Address: "10.0.0.1", Port: 8080, Source: "podman"},
			wantErr:   false,
		},
		{
			name:      "empty ID",
			svcName:   "myapp",
			namespace: "default",
			instance:  &Instance{ID: "", Address: "10.0.0.1", Port: 8080},
			wantErr:   true,
		},
		{
			name:      "empty address",
			svcName:   "myapp",
			namespace: "default",
			instance:  &Instance{ID: "a1b2c3", Address: "", Port: 8080},
			wantErr:   true,
		},
		{
			name:      "zero port",
			svcName:   "myapp",
			namespace: "default",
			instance:  &Instance{ID: "a1b2c3", Address: "10.0.0.1", Port: 0},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore()
			err := s.Register(tt.svcName, tt.namespace, tt.instance)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStore_Register_DefaultsApplied(t *testing.T) {
	s := NewStore()
	inst := &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "podman"}
	if err := s.Register("myapp", "default", inst); err != nil {
		t.Fatal(err)
	}

	got, ok := s.GetInstance("myapp", "default", "a1")
	if !ok {
		t.Fatal("instance not found")
	}

	if got.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want %q", got.Protocol, "tcp")
	}
	if got.Priority != 10 {
		t.Errorf("Priority = %d, want %d", got.Priority, 10)
	}
	if got.Weight != 100 {
		t.Errorf("Weight = %d, want %d", got.Weight, 100)
	}
}

func TestStore_Register_UpdateExisting(t *testing.T) {
	s := NewStore()
	inst1 := &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "podman"}
	inst2 := &Instance{ID: "a1", Address: "10.0.0.2", Port: 9090, Source: "podman"}

	if err := s.Register("myapp", "default", inst1); err != nil {
		t.Fatal(err)
	}
	if err := s.Register("myapp", "default", inst2); err != nil {
		t.Fatal(err)
	}

	instances := s.GetInstances("myapp", "default")
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}

	got := instances[0]
	if got.Address != "10.0.0.2" {
		t.Errorf("Address = %q, want %q", got.Address, "10.0.0.2")
	}
	if got.Port != 9090 {
		t.Errorf("Port = %d, want %d", got.Port, 9090)
	}
}

func TestStore_Register_MultipleInstances(t *testing.T) {
	s := NewStore()
	for i := 0; i < 3; i++ {
		inst := &Instance{
			ID:      fmt.Sprintf("inst%d", i),
			Address: fmt.Sprintf("10.0.0.%d", i+1),
			Port:    8080,
			Source:  "podman",
		}
		if err := s.Register("myapp", "default", inst); err != nil {
			t.Fatal(err)
		}
	}

	instances := s.GetInstances("myapp", "default")
	if len(instances) != 3 {
		t.Fatalf("expected 3 instances, got %d", len(instances))
	}
}

func TestStore_Deregister(t *testing.T) {
	s := NewStore()
	inst := &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "podman"}
	if err := s.Register("myapp", "default", inst); err != nil {
		t.Fatal(err)
	}

	s.Deregister("myapp", "default", "a1")

	if _, ok := s.GetInstance("myapp", "default", "a1"); ok {
		t.Error("instance should be removed")
	}
	if _, ok := s.GetService("myapp", "default"); ok {
		t.Error("empty service should be cleaned up")
	}
}

func TestStore_Deregister_NonExistent(t *testing.T) {
	s := NewStore()
	s.Deregister("nonexistent", "default", "x1")
}

func TestStore_GetInstance_NotFound(t *testing.T) {
	s := NewStore()
	if _, ok := s.GetInstance("myapp", "default", "x1"); ok {
		t.Error("expected not found")
	}
}

func TestStore_GetInstances_Empty(t *testing.T) {
	s := NewStore()
	if instances := s.GetInstances("myapp", "default"); len(instances) != 0 {
		t.Errorf("expected 0 instances, got %d", len(instances))
	}
}

func TestStore_ListServices(t *testing.T) {
	s := NewStore()
	s.Register("svc1", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "podman"})
	s.Register("svc2", "default", &Instance{ID: "b1", Address: "10.0.0.2", Port: 9090, Source: "qemu"})
	s.Register("svc3", "prod", &Instance{ID: "c1", Address: "10.0.0.3", Port: 443, Source: "qemu"})

	services := s.ListServices()
	if len(services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(services))
	}
}

func TestStore_DeregisterBySource(t *testing.T) {
	s := NewStore()
	s.Register("svc1", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "podman"})
	s.Register("svc1", "default", &Instance{ID: "a2", Address: "10.0.0.2", Port: 8080, Source: "qemu"})
	s.Register("svc2", "default", &Instance{ID: "b1", Address: "10.0.0.3", Port: 9090, Source: "podman"})

	s.DeregisterBySource("podman")

	instances := s.GetInstances("svc1", "default")
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance (qemu), got %d", len(instances))
	}
	if instances[0].Source != "qemu" {
		t.Errorf("expected qemu source, got %s", instances[0].Source)
	}

	if _, ok := s.GetService("svc2", "default"); ok {
		t.Error("svc2 should be removed (all instances were podman)")
	}
}

func TestStore_Concurrent(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			s.Register("myapp", "default", &Instance{
				ID:      fmt.Sprintf("inst%d", i),
				Address: fmt.Sprintf("10.0.0.%d", i),
				Port:    8080,
				Source:  "podman",
			})
		}(i)
		go func(i int) {
			defer wg.Done()
			s.GetInstances("myapp", "default")
		}(i)
	}

	wg.Wait()

	if len(s.GetInstances("myapp", "default")) != 100 {
		t.Errorf("expected 100 instances, got %d", len(s.GetInstances("myapp", "default")))
	}
}

func TestStore_ServiceKey_EmptyNamespace(t *testing.T) {
	key := serviceKey("myapp", "")
	if key != "myapp.default" {
		t.Errorf("expected 'myapp.default', got %q", key)
	}
}

func TestStore_GetService_NotFound(t *testing.T) {
	s := NewStore()
	if _, ok := s.GetService("nonexistent", "default"); ok {
		t.Error("expected not found")
	}
}

func TestStore_GetService_EmptyNamespace(t *testing.T) {
	s := NewStore()
	s.Register("myapp", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"})
	if _, ok := s.GetService("myapp", ""); !ok {
		t.Error("expected service found with empty namespace")
	}
}

func TestStore_Deregister_EmptyNamespace(t *testing.T) {
	s := NewStore()
	s.Register("myapp", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"})
	s.Deregister("myapp", "", "a1")
	if _, ok := s.GetInstance("myapp", "default", "a1"); ok {
		t.Error("instance should be removed")
	}
}

func TestStore_GetInstances_EmptyNamespace(t *testing.T) {
	s := NewStore()
	s.Register("myapp", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"})
	if len(s.GetInstances("myapp", "")) != 1 {
		t.Error("expected 1 instance with empty namespace")
	}
}

func TestStore_GetInstance_EmptyNamespace(t *testing.T) {
	s := NewStore()
	s.Register("myapp", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"})
	if _, ok := s.GetInstance("myapp", "", "a1"); !ok {
		t.Error("expected instance found with empty namespace")
	}
}
