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
		{
			name:      "negative port",
			svcName:   "myapp",
			namespace: "default",
			instance:  &Instance{ID: "a1b2c3", Address: "10.0.0.1", Port: -1},
			wantErr:   true,
		},
		{
			name:      "port too high",
			svcName:   "myapp",
			namespace: "default",
			instance:  &Instance{ID: "a1b2c3", Address: "10.0.0.1", Port: 65536},
			wantErr:   true,
		},
		{
			name:      "port at upper bound",
			svcName:   "myapp",
			namespace: "default",
			instance:  &Instance{ID: "a1b2c3", Address: "10.0.0.1", Port: 65535},
			wantErr:   false,
		},
		{
			name:      "invalid address",
			svcName:   "myapp",
			namespace: "default",
			instance:  &Instance{ID: "a1b2c3", Address: "not-an-ip", Port: 8080},
			wantErr:   true,
		},
		{
			name:      "ipv6 address",
			svcName:   "myapp",
			namespace: "default",
			instance:  &Instance{ID: "a1b2c3", Address: "::1", Port: 8080},
			wantErr:   false,
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

func TestStore_Deregister_NonExistent(_ *testing.T) {
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
	for _, svc := range []struct {
		name, ns, id, addr string
		port               int
		src                string
	}{
		{"svc1", "default", "a1", "10.0.0.1", 8080, "podman"},
		{"svc2", "default", "b1", "10.0.0.2", 9090, "qemu"},
		{"svc3", "prod", "c1", "10.0.0.3", 443, "qemu"},
	} {
		if err := s.Register(svc.name, svc.ns, &Instance{ID: svc.id, Address: svc.addr, Port: svc.port, Source: svc.src}); err != nil {
			t.Fatal(err)
		}
	}

	services := s.ListServices()
	if len(services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(services))
	}
}

func TestStore_DeregisterBySource(t *testing.T) {
	s := NewStore()
	for _, inst := range []struct {
		svc, ns, id, addr, src string
		port                   int
	}{
		{"svc1", "default", "a1", "10.0.0.1", "podman", 8080},
		{"svc1", "default", "a2", "10.0.0.2", "qemu", 8080},
		{"svc2", "default", "b1", "10.0.0.3", "podman", 9090},
	} {
		if err := s.Register(inst.svc, inst.ns, &Instance{ID: inst.id, Address: inst.addr, Port: inst.port, Source: inst.src}); err != nil {
			t.Fatal(err)
		}
	}

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
			_ = s.Register("myapp", "default", &Instance{
				ID:      fmt.Sprintf("inst%d", i),
				Address: fmt.Sprintf("10.0.0.%d", i),
				Port:    8080,
				Source:  "podman",
			})
		}(i)
		go func(_ int) {
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
	if err := s.Register("myapp", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.GetService("myapp", ""); !ok {
		t.Error("expected service found with empty namespace")
	}
}

func TestStore_Deregister_EmptyNamespace(t *testing.T) {
	s := NewStore()
	if err := s.Register("myapp", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	s.Deregister("myapp", "", "a1")
	if _, ok := s.GetInstance("myapp", "default", "a1"); ok {
		t.Error("instance should be removed")
	}
}

func TestStore_GetInstances_EmptyNamespace(t *testing.T) {
	s := NewStore()
	if err := s.Register("myapp", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if len(s.GetInstances("myapp", "")) != 1 {
		t.Error("expected 1 instance with empty namespace")
	}
}

func TestStore_GetInstance_EmptyNamespace(t *testing.T) {
	s := NewStore()
	if err := s.Register("myapp", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.GetInstance("myapp", "", "a1"); !ok {
		t.Error("expected instance found with empty namespace")
	}
}

func TestStore_Register_DoesNotMutateInput(t *testing.T) {
	s := NewStore()
	inst := &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"}

	if err := s.Register("myapp", "default", inst); err != nil {
		t.Fatal(err)
	}

	if inst.Protocol != "" {
		t.Errorf("input instance Protocol was mutated: got %q, want %q", inst.Protocol, "")
	}
	if inst.Priority != 0 {
		t.Errorf("input instance Priority was mutated: got %d, want %d", inst.Priority, 0)
	}
	if inst.Weight != 0 {
		t.Errorf("input instance Weight was mutated: got %d, want %d", inst.Weight, 0)
	}
}

func TestStore_Register_InvalidServiceName(t *testing.T) {
	s := NewStore()
	for _, name := range []string{"", "a b", "a.b", "a_b", "-foo", "foo-", string(make([]byte, 64))} {
		err := s.Register(name, "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"})
		if err == nil {
			t.Errorf("expected error for invalid service name %q", name)
		}
	}
}

func TestStore_Register_InvalidNamespace(t *testing.T) {
	s := NewStore()
	for _, ns := range []string{"a b", "a.b", "a_b", "-foo", "foo-"} {
		err := s.Register("myapp", ns, &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"})
		if err == nil {
			t.Errorf("expected error for invalid namespace %q", ns)
		}
	}
}

func TestStore_Register_InvalidInstanceID(t *testing.T) {
	s := NewStore()
	for _, id := range []string{"a b", "a.b", "a_b", "-foo", "foo-"} {
		err := s.Register("myapp", "default", &Instance{ID: id, Address: "10.0.0.1", Port: 8080, Source: "test"})
		if err == nil {
			t.Errorf("expected error for invalid instance ID %q", id)
		}
	}
}

func TestStore_Register_ValidNamesWithHyphens(t *testing.T) {
	s := NewStore()
	for _, name := range []string{"open-webui", "web-frontend-https", "a", "svc1", "my-app-2"} {
		err := s.Register(name, "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"})
		if err != nil {
			t.Errorf("unexpected error for valid service name %q: %v", name, err)
		}
	}
}

func TestStore_GetInstance_ReturnsCopy(t *testing.T) {
	s := NewStore()
	if err := s.Register("myapp", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"}); err != nil {
		t.Fatal(err)
	}

	got, ok := s.GetInstance("myapp", "default", "a1")
	if !ok {
		t.Fatal("instance not found")
	}
	got.Address = "10.0.0.999"
	got.Port = 12345

	got2, ok := s.GetInstance("myapp", "default", "a1")
	if !ok {
		t.Fatal("instance not found on second call")
	}
	if got2.Address != "10.0.0.1" {
		t.Errorf("internal instance was mutated via returned pointer: Address = %q, want %q", got2.Address, "10.0.0.1")
	}
	if got2.Port != 8080 {
		t.Errorf("internal instance was mutated via returned pointer: Port = %d, want %d", got2.Port, 8080)
	}
}

func TestStore_GetInstances_ReturnsCopy(t *testing.T) {
	s := NewStore()
	if err := s.Register("myapp", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"}); err != nil {
		t.Fatal(err)
	}

	instances := s.GetInstances("myapp", "default")
	instances[0].Address = "10.0.0.999"

	got, _ := s.GetInstance("myapp", "default", "a1")
	if got.Address != "10.0.0.1" {
		t.Errorf("internal instance was mutated via returned slice: Address = %q, want %q", got.Address, "10.0.0.1")
	}
}

func TestStore_GetService_ReturnsCopy(t *testing.T) {
	s := NewStore()
	if err := s.Register("myapp", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"}); err != nil {
		t.Fatal(err)
	}

	svc, ok := s.GetService("myapp", "default")
	if !ok {
		t.Fatal("service not found")
	}
	svc.Instances["a1"].Address = "10.0.0.999"

	got, _ := s.GetInstance("myapp", "default", "a1")
	if got.Address != "10.0.0.1" {
		t.Errorf("internal instance was mutated via returned service: Address = %q, want %q", got.Address, "10.0.0.1")
	}
}

func TestStore_ListServices_ReturnsCopy(t *testing.T) {
	s := NewStore()
	if err := s.Register("myapp", "default", &Instance{ID: "a1", Address: "10.0.0.1", Port: 8080, Source: "test"}); err != nil {
		t.Fatal(err)
	}

	services := s.ListServices()
	for _, svc := range services {
		if svc.Name == "myapp" {
			svc.Instances["a1"].Address = "10.0.0.999"
		}
	}

	got, _ := s.GetInstance("myapp", "default", "a1")
	if got.Address != "10.0.0.1" {
		t.Errorf("internal instance was mutated via returned list: Address = %q, want %q", got.Address, "10.0.0.1")
	}
}

func TestStore_CopyHelpers_Nil(t *testing.T) {
	if copyInstance(nil) != nil {
		t.Error("copyInstance(nil) should return nil")
	}
	if copyService(nil) != nil {
		t.Error("copyService(nil) should return nil")
	}
}
