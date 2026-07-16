package discovery

import (
	"fmt"
	"sync"
)

// Instance represents a single instance of a discovered service.
type Instance struct {
	ID       string `json:"id"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
	Source   string `json:"source"`
}

// Service represents a service with its instances.
type Service struct {
	Name      string               `json:"name"`
	Namespace string               `json:"namespace"`
	Instances map[string]*Instance `json:"instances"`
}

// Store is a thread-safe in-memory store for discovered services.
type Store struct {
	mu       sync.RWMutex
	services map[string]*Service
}

// NewStore creates a new Store.
func NewStore() *Store {
	return &Store{
		services: make(map[string]*Service),
	}
}

func serviceKey(name, namespace string) string {
	if namespace == "" {
		namespace = "default"
	}
	return fmt.Sprintf("%s.%s", name, namespace)
}

// Register adds or updates an instance of a service in the store.
func (s *Store) Register(svcName, namespace string, inst *Instance) error {
	if namespace == "" {
		namespace = "default"
	}

	if !isDNSLabelSafe(svcName) {
		return fmt.Errorf("invalid service name %q: must be a valid DNS label (alphanumeric and hyphens, max 63 chars)", svcName)
	}
	if !isDNSLabelSafe(namespace) {
		return fmt.Errorf("invalid namespace %q: must be a valid DNS label (alphanumeric and hyphens, max 63 chars)", namespace)
	}
	if inst.ID == "" {
		return fmt.Errorf("instance ID cannot be empty")
	}
	if !isDNSLabelSafe(inst.ID) {
		return fmt.Errorf("invalid instance ID %q: must be a valid DNS label (alphanumeric and hyphens, max 63 chars)", inst.ID)
	}
	if inst.Address == "" {
		return fmt.Errorf("instance address cannot be empty")
	}
	if inst.Port == 0 {
		return fmt.Errorf("instance port cannot be 0")
	}

	// Copy to avoid mutating the caller's instance when applying defaults.
	inst = &Instance{
		ID:       inst.ID,
		Address:  inst.Address,
		Port:     inst.Port,
		Protocol: inst.Protocol,
		Priority: inst.Priority,
		Weight:   inst.Weight,
		Source:   inst.Source,
	}

	if inst.Protocol == "" {
		inst.Protocol = "tcp"
	}
	if inst.Priority == 0 {
		inst.Priority = 10
	}
	if inst.Weight == 0 {
		inst.Weight = 100
	}

	key := serviceKey(svcName, namespace)

	s.mu.Lock()
	defer s.mu.Unlock()

	svc, ok := s.services[key]
	if !ok {
		svc = &Service{
			Name:      svcName,
			Namespace: namespace,
			Instances: make(map[string]*Instance),
		}
		s.services[key] = svc
	}

	svc.Instances[inst.ID] = inst
	return nil
}

// isDNSLabelSafe reports whether s is a valid DNS label:
// 1-63 characters, alphanumeric or hyphens, not starting or ending with a hyphen.
func isDNSLabelSafe(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		isAlpha := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		isDigit := c >= '0' && c <= '9'
		isHyphen := c == '-'
		if !isAlpha && !isDigit && !isHyphen {
			return false
		}
		if isHyphen && (i == 0 || i == len(s)-1) {
			return false
		}
	}
	return true
}

// Deregister removes an instance from a service.
func (s *Store) Deregister(svcName, namespace, instanceID string) {
	if namespace == "" {
		namespace = "default"
	}

	key := serviceKey(svcName, namespace)

	s.mu.Lock()
	defer s.mu.Unlock()

	svc, ok := s.services[key]
	if !ok {
		return
	}

	delete(svc.Instances, instanceID)

	if len(svc.Instances) == 0 {
		delete(s.services, key)
	}
}

// GetService returns a service and all its instances.
func (s *Store) GetService(svcName, namespace string) (*Service, bool) {
	if namespace == "" {
		namespace = "default"
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	svc, ok := s.services[serviceKey(svcName, namespace)]
	if !ok {
		return nil, false
	}
	return svc, true
}

// GetInstances returns all instances of a service.
func (s *Store) GetInstances(svcName, namespace string) []*Instance {
	if namespace == "" {
		namespace = "default"
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	svc, ok := s.services[serviceKey(svcName, namespace)]
	if !ok {
		return nil
	}

	instances := make([]*Instance, 0, len(svc.Instances))
	for _, inst := range svc.Instances {
		instances = append(instances, inst)
	}
	return instances
}

// GetInstance returns a specific instance of a service.
func (s *Store) GetInstance(svcName, namespace, instanceID string) (*Instance, bool) {
	if namespace == "" {
		namespace = "default"
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	svc, ok := s.services[serviceKey(svcName, namespace)]
	if !ok {
		return nil, false
	}

	inst, ok := svc.Instances[instanceID]
	return inst, ok
}

// ListServices returns all services in the store.
func (s *Store) ListServices() []*Service {
	s.mu.RLock()
	defer s.mu.RUnlock()

	services := make([]*Service, 0, len(s.services))
	for _, svc := range s.services {
		services = append(services, svc)
	}
	return services
}

// DeregisterBySource removes all instances discovered by a specific source.
func (s *Store) DeregisterBySource(source string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, svc := range s.services {
		for id, inst := range svc.Instances {
			if inst.Source == source {
				delete(svc.Instances, id)
			}
		}
		if len(svc.Instances) == 0 {
			delete(s.services, key)
		}
	}
}
