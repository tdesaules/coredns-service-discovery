// Package discovery implements a CoreDNS plugin for DNS-based service discovery.
package discovery

import (
	"context"
	"fmt"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Handler implements plugin.Handler and serves DNS records
// from the in-memory store.
type Handler struct {
	Next  plugin.Handler
	Store *Store
	Zone  string
	TTL   uint32
}

// Name implements plugin.Handler.
func (h *Handler) Name() string { return "discovery" }

// ServeDNS implements plugin.Handler.
// It parses the query name, looks up the store, and returns A or SRV records.
// If no match is found, it falls through to the next plugin.
func (h *Handler) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	zone := strings.ToLower(h.Zone)
	qname := strings.ToLower(state.QName())

	if !strings.HasSuffix(qname, zone) {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}

	rel := strings.TrimSuffix(qname, zone)
	rel = strings.TrimSuffix(rel, ".")
	if rel == "" {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}

	labels := strings.Split(rel, ".")
	if len(labels) < 2 {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}

	namespace := labels[len(labels)-1]

	switch state.QType() {
	case dns.TypeA:
		return h.serveA(w, r, labels, namespace)
	case dns.TypeSRV:
		return h.serveSRV(w, r, labels, namespace)
	default:
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}
}

// serveA handles A record queries.
// Supported patterns:
//
//	<service>.<namespace>.<zone>          → all instance IPs
//	<instance>.<service>.<namespace>.<zone> → specific instance IP
func (h *Handler) serveA(w dns.ResponseWriter, r *dns.Msg, labels []string, namespace string) (int, error) {
	var svcName, instanceID string

	if len(labels) >= 3 {
		instanceID = labels[0]
		svcName = labels[1]
	} else {
		svcName = labels[0]
	}

	var instances []*Instance
	if instanceID != "" {
		inst, ok := h.Store.GetInstance(svcName, namespace, instanceID)
		if !ok {
			return h.nxdomain(w, r)
		}
		instances = []*Instance{inst}
	} else {
		instances = h.Store.GetInstances(svcName, namespace)
		if len(instances) == 0 {
			return h.nxdomain(w, r)
		}
	}

	rrs := make([]dns.RR, 0, len(instances))
	for _, inst := range instances {
		rr, err := dns.NewRR(fmt.Sprintf("%s %d IN A %s", r.Question[0].Name, h.TTL, inst.Address))
		if err != nil {
			continue
		}
		rrs = append(rrs, rr)
	}

	if len(rrs) == 0 {
		return h.nxdomain(w, r)
	}

	resp := new(dns.Msg)
	resp.SetReply(r)
	resp.Answer = rrs
	resp.Authoritative = true
	if err := w.WriteMsg(resp); err != nil {
		return dns.RcodeServerFailure, err
	}
	return dns.RcodeSuccess, nil
}

// serveSRV handles SRV record queries.
// Supported pattern:
//
//	_<service>._<proto>.<namespace>.<zone> → SRV per instance
func (h *Handler) serveSRV(w dns.ResponseWriter, r *dns.Msg, labels []string, namespace string) (int, error) {
	if len(labels) < 3 {
		return h.nxdomain(w, r)
	}

	if !strings.HasPrefix(labels[0], "_") || !strings.HasPrefix(labels[1], "_") {
		return h.nxdomain(w, r)
	}

	svcName := strings.TrimPrefix(labels[0], "_")
	// proto := strings.TrimPrefix(labels[1], "_") // "tcp" or "udp"

	instances := h.Store.GetInstances(svcName, namespace)
	if len(instances) == 0 {
		return h.nxdomain(w, r)
	}

	rrs := make([]dns.RR, 0, len(instances))
	for _, inst := range instances {
		target := fmt.Sprintf("%s.%s.%s.%s", inst.ID, svcName, namespace, h.Zone)
		rr, err := dns.NewRR(fmt.Sprintf("%s %d IN SRV %d %d %d %s",
			r.Question[0].Name, h.TTL, inst.Priority, inst.Weight, inst.Port, target))
		if err != nil {
			continue
		}
		rrs = append(rrs, rr)
	}

	if len(rrs) == 0 {
		return h.nxdomain(w, r)
	}

	resp := new(dns.Msg)
	resp.SetReply(r)
	resp.Answer = rrs
	resp.Authoritative = true
	if err := w.WriteMsg(resp); err != nil {
		return dns.RcodeServerFailure, err
	}
	return dns.RcodeSuccess, nil
}

// nxdomain writes an NXDOMAIN response.
func (h *Handler) nxdomain(w dns.ResponseWriter, r *dns.Msg) (int, error) {
	resp := new(dns.Msg)
	resp.SetReply(r)
	resp.Rcode = dns.RcodeNameError
	resp.Authoritative = true
	if err := w.WriteMsg(resp); err != nil {
		return dns.RcodeServerFailure, err
	}
	return dns.RcodeNameError, nil
}
