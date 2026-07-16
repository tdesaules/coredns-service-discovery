package discovery

import (
	"context"
	"sync"

	"github.com/coredns/caddy"
)

// Source is a discovery source that populates the store.
// Each source (podman, qemu, etc.) implements this interface
// and registers itself via RegisterSource in its init() function.
type Source interface {
	// Name returns the source name (e.g., "podman", "qemu").
	Name() string

	// ParseConfig parses source-specific configuration from the Corefile.
	// The controller cursor is positioned after the "{" token.
	// ParseConfig should consume tokens using c.Next() until it
	// encounters the closing "}" of the source sub-block.
	ParseConfig(c *caddy.Controller) error

	// Run starts the discovery loop. It populates the store and keeps
	// it updated. Blocks until ctx is cancelled.
	Run(ctx context.Context, store *Store) error
}

// SourceFactory creates a new Source instance.
type SourceFactory func() Source

var (
	sourceRegistry   = map[string]SourceFactory{}
	sourceRegistryMu sync.RWMutex
)

// RegisterSource registers a source factory.
// Called from init() in each source_*.go file.
func RegisterSource(name string, factory SourceFactory) {
	sourceRegistryMu.Lock()
	defer sourceRegistryMu.Unlock()
	sourceRegistry[name] = factory
}

// GetSource returns a source factory by name.
func GetSource(name string) (SourceFactory, bool) {
	sourceRegistryMu.RLock()
	defer sourceRegistryMu.RUnlock()
	f, ok := sourceRegistry[name]
	return f, ok
}

// RegisteredSources returns the names of all registered sources.
func RegisteredSources() []string {
	sourceRegistryMu.RLock()
	defer sourceRegistryMu.RUnlock()
	names := make([]string, 0, len(sourceRegistry))
	for name := range sourceRegistry {
		names = append(names, name)
	}
	return names
}
