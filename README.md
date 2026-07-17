# discovery

## Name

*discovery* - DNS-based service discovery with pluggable sources.

## Description

*discovery* provides DNS-based service discovery for CoreDNS. It maintains an in-memory store of services and their instances, populated by pluggable sources (Podman, QEMU, etc.). The plugin serves A and SRV DNS records for the configured zone, allowing clients (such as Caddy) to discover service instances via standard DNS queries.

The plugin follows the *List + Watch* pattern (similar to the *kubernetes* plugin): sources perform an initial list of available services, then watch for changes in real-time, keeping the in-memory store synchronized.

**Status:** Core framework (store, handler, source interface, setup) is implemented and tested. The Podman source is the next planned implementation. QEMU source is planned after that.

## Syntax

```
discovery ZONE {
    ttl SECONDS
    fallthrough
    source NAME {
        # source-specific configuration
    }
}
```

* `ttl` — TTL for DNS responses in seconds. Default: 30. Maximum: 3600.
* `fallthrough` — If a query matches the zone but no record is found, pass the request to the next plugin instead of returning NXDOMAIN.
* `source` — Configures a discovery source. Can be specified multiple times. Each source maintains its own discovery loop and feeds into the shared store.

## DNS Convention

The plugin serves the following record types for the configured zone:

### A Records

```
<service>.<namespace>.<zone>              → IPs of all instances
<instance-id>.<service>.<namespace>.<zone> → IP of a specific instance
```

### SRV Records (RFC 2782)

```
_<service>._<proto>.<namespace>.<zone>    → priority weight port <instance-id>.<service>.<namespace>.<zone>
```

### Example

```
open-webui.default.svc.desaules.in.                    A      10.88.0.5
_open-webui._tcp.default.svc.desaules.in.              SRV    10 100 8080 a1b2c3.open-webui.default.svc.desaules.in.
a1b2c3.open-webui.default.svc.desaules.in.             A      10.88.0.5
```

## Sources

### podman

Discovers Podman containers via the Podman REST API (Unix socket). Uses the *List + Watch* pattern: initial container list followed by real-time event streaming.

Container labels:

| Label | Required | Description |
|---|---|---|
| `discovery.enable` | Yes | Must be `true` to enable discovery |
| `discovery.service` | Yes | Service name |
| `discovery.port` | Yes | Service port |
| `discovery.protocol` | No | `tcp` (default) or `udp` |
| `discovery.namespace` | No | Namespace (default: `default`) |

Configuration:

```
source podman {
    socket /run/podman/podman.sock     # Podman API socket
    host_ip 10.10.10.1                  # IP for host-networked containers
    refresh 30s                         # Resync interval (fallback)
    label discovery.enable=true         # Container label filter
}
```

### qemu (planned)

Discovers QEMU virtual machines via QMP (QEMU Machine Protocol). Uses guest agent for IP discovery.

## Examples

```
svc.desaules.in {
    discovery svc.desaules.in {
        ttl 30
        source podman {
            socket /run/podman/podman.sock
            host_ip 10.10.10.1
        }
    }
    cache 30
    loadbalance
    errors
    forward . /etc/resolv.conf
}
```

## Adding New Sources

A source is a Go type implementing the `Source` interface:

```go
type Source interface {
    Name() string
    ParseConfig(c *caddy.Controller) error
    Run(ctx context.Context, store *Store) error
}
```

To register a new source, create a file `source_<name>.go` with:

```go
func init() {
    RegisterSource("<name>", func() Source { return &MySource{} })
}
```

The source's `Run` method should populate the store via `store.Register` and `store.Deregister`, and block until `ctx` is cancelled.

## Integration with CoreDNS

This plugin is an external CoreDNS plugin. It is compiled into CoreDNS via a [separate integration repo](https://github.com/tdesaules/coredns) that uses [Method 2](https://coredns.io/2017/07/25/compile-time-enabling-or-disabling-plugins/) (external Go program importing CoreDNS as a library). No fork of CoreDNS is required.

Pre-built Docker image: `ghcr.io/tdesaules/coredns:latest`

## See Also

- [CoreDNS Plugins](https://coredns.io/plugins/)
- [Writing Plugins for CoreDNS](https://coredns.io/2016/12/19/writing-plugins-for-coredns/)
- [RFC 2782](https://tools.ietf.org/html/rfc2782) - SRV Resource Record
