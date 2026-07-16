package discovery

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func setupTestHandler(store *Store) *Handler {
	return &Handler{
		Store: store,
		Zone:  "svc.desaules.in.",
		TTL:   30,
	}
}

func populateStore(s *Store) {
	if err := s.Register("open-webui", "default", &Instance{
		ID: "a1b2c3", Address: "10.88.0.5", Port: 8080, Source: "podman",
	}); err != nil {
		panic(err)
	}
	if err := s.Register("open-webui", "default", &Instance{
		ID: "d4e5f6", Address: "10.88.0.6", Port: 8080, Source: "podman",
	}); err != nil {
		panic(err)
	}
	if err := s.Register("web-frontend-https", "default", &Instance{
		ID: "vm1", Address: "10.10.10.5", Port: 443, Source: "qemu",
	}); err != nil {
		panic(err)
	}
}

func TestHandler_ARecord_ServiceLevel(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)

	req := test.Case{
		Qname: "open-webui.default.svc.desaules.in.",
		Qtype: dns.TypeA,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err := h.ServeDNS(context.Background(), rec, req)
	if err != nil {
		t.Fatalf("ServeDNS returned error: %v", err)
	}

	if len(rec.Msg.Answer) != 2 {
		t.Fatalf("expected 2 A records, got %d", len(rec.Msg.Answer))
	}

	for _, rr := range rec.Msg.Answer {
		if _, ok := rr.(*dns.A); !ok {
			t.Errorf("expected *dns.A, got %T", rr)
		}
	}
}

func TestHandler_ARecord_InstanceLevel(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)

	req := test.Case{
		Qname: "a1b2c3.open-webui.default.svc.desaules.in.",
		Qtype: dns.TypeA,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err := h.ServeDNS(context.Background(), rec, req)
	if err != nil {
		t.Fatalf("ServeDNS returned error: %v", err)
	}

	if len(rec.Msg.Answer) != 1 {
		t.Fatalf("expected 1 A record, got %d", len(rec.Msg.Answer))
	}

	a := rec.Msg.Answer[0].(*dns.A)
	if a.A.String() != "10.88.0.5" {
		t.Errorf("expected 10.88.0.5, got %s", a.A.String())
	}
}

func TestHandler_ARecord_NonExistent(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)

	req := test.Case{
		Qname: "nonexistent.default.svc.desaules.in.",
		Qtype: dns.TypeA,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	code, _ := h.ServeDNS(context.Background(), rec, req)

	if code != dns.RcodeNameError {
		t.Errorf("expected NXDOMAIN, got rcode %d", code)
	}
}

func TestHandler_SRVRecord(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)

	req := test.Case{
		Qname: "_open-webui._tcp.default.svc.desaules.in.",
		Qtype: dns.TypeSRV,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err := h.ServeDNS(context.Background(), rec, req)
	if err != nil {
		t.Fatalf("ServeDNS returned error: %v", err)
	}

	if len(rec.Msg.Answer) != 2 {
		t.Fatalf("expected 2 SRV records, got %d", len(rec.Msg.Answer))
	}

	for _, rr := range rec.Msg.Answer {
		srv, ok := rr.(*dns.SRV)
		if !ok {
			t.Errorf("expected *dns.SRV, got %T", rr)
			continue
		}
		if srv.Port != 8080 {
			t.Errorf("expected port 8080, got %d", srv.Port)
		}
	}
}

func TestHandler_SRVRecord_QEMU(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)

	req := test.Case{
		Qname: "_web-frontend-https._tcp.default.svc.desaules.in.",
		Qtype: dns.TypeSRV,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err := h.ServeDNS(context.Background(), rec, req)
	if err != nil {
		t.Fatalf("ServeDNS returned error: %v", err)
	}

	if len(rec.Msg.Answer) != 1 {
		t.Fatalf("expected 1 SRV record, got %d", len(rec.Msg.Answer))
	}

	srv := rec.Msg.Answer[0].(*dns.SRV)
	if srv.Port != 443 {
		t.Errorf("expected port 443, got %d", srv.Port)
	}
}

func TestHandler_OutsideZone(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)
	h.Next = test.NextHandler(dns.RcodeSuccess, nil)

	req := test.Case{
		Qname: "example.com.",
		Qtype: dns.TypeA,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	code, _ := h.ServeDNS(context.Background(), rec, req)

	if code != dns.RcodeSuccess {
		t.Errorf("expected fallthrough to next plugin, got rcode %d", code)
	}
}

func TestHandler_UnsupportedQueryType(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)
	h.Next = test.NextHandler(dns.RcodeSuccess, nil)

	req := test.Case{
		Qname: "open-webui.default.svc.desaules.in.",
		Qtype: dns.TypeMX,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	code, _ := h.ServeDNS(context.Background(), rec, req)

	if code != dns.RcodeSuccess {
		t.Errorf("expected fallthrough for MX query, got rcode %d", code)
	}
}

func TestHandler_ARecord_QVMInstance(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)

	req := test.Case{
		Qname: "vm1.web-frontend-https.default.svc.desaules.in.",
		Qtype: dns.TypeA,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err := h.ServeDNS(context.Background(), rec, req)
	if err != nil {
		t.Fatalf("ServeDNS returned error: %v", err)
	}

	if len(rec.Msg.Answer) != 1 {
		t.Fatalf("expected 1 A record, got %d", len(rec.Msg.Answer))
	}

	a := rec.Msg.Answer[0].(*dns.A)
	if a.A.String() != "10.10.10.5" {
		t.Errorf("expected 10.10.10.5, got %s", a.A.String())
	}
}

func TestHandler_Authoritative(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)

	req := test.Case{
		Qname: "open-webui.default.svc.desaules.in.",
		Qtype: dns.TypeA,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	if _, err := h.ServeDNS(context.Background(), rec, req); err != nil {
		t.Fatalf("ServeDNS returned error: %v", err)
	}

	if !rec.Msg.Authoritative {
		t.Error("response should be authoritative")
	}
}

func TestHandler_TTLInResponse(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)
	h.TTL = 60

	req := test.Case{
		Qname: "open-webui.default.svc.desaules.in.",
		Qtype: dns.TypeA,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	if _, err := h.ServeDNS(context.Background(), rec, req); err != nil {
		t.Fatalf("ServeDNS returned error: %v", err)
	}

	for _, rr := range rec.Msg.Answer {
		if rr.Header().Ttl != 60 {
			t.Errorf("expected TTL 60, got %d", rr.Header().Ttl)
		}
	}
}

func TestHandler_ZoneApex(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)
	h.Next = test.NextHandler(dns.RcodeSuccess, nil)

	req := test.Case{
		Qname: "svc.desaules.in.",
		Qtype: dns.TypeA,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	code, _ := h.ServeDNS(context.Background(), rec, req)

	if code != dns.RcodeSuccess {
		t.Errorf("expected fallthrough for zone apex, got rcode %d", code)
	}
}

func TestHandler_SingleLabel(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)
	h.Next = test.NextHandler(dns.RcodeSuccess, nil)

	req := test.Case{
		Qname: "foo.svc.desaules.in.",
		Qtype: dns.TypeA,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	code, _ := h.ServeDNS(context.Background(), rec, req)

	if code != dns.RcodeSuccess {
		t.Errorf("expected fallthrough for single label, got rcode %d", code)
	}
}

func TestHandler_ARecord_InstanceNotFound(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)

	req := test.Case{
		Qname: "nonexistent.open-webui.default.svc.desaules.in.",
		Qtype: dns.TypeA,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	code, _ := h.ServeDNS(context.Background(), rec, req)

	if code != dns.RcodeNameError {
		t.Errorf("expected NXDOMAIN, got rcode %d", code)
	}
}

func TestHandler_ARecord_InvalidAddress(t *testing.T) {
	store := NewStore()
	if err := store.Register("bad-svc", "default", &Instance{
		ID: "bad1", Address: "not-an-ip", Port: 8080, Source: "test",
	}); err != nil {
		t.Fatal(err)
	}
	h := setupTestHandler(store)

	req := test.Case{
		Qname: "bad-svc.default.svc.desaules.in.",
		Qtype: dns.TypeA,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	code, _ := h.ServeDNS(context.Background(), rec, req)

	if code != dns.RcodeNameError {
		t.Errorf("expected NXDOMAIN for invalid address, got rcode %d", code)
	}
}

func TestHandler_SRVRecord_TwoLabels(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)

	req := test.Case{
		Qname: "_open-webui.default.svc.desaules.in.",
		Qtype: dns.TypeSRV,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	code, _ := h.ServeDNS(context.Background(), rec, req)

	if code != dns.RcodeNameError {
		t.Errorf("expected NXDOMAIN for 2-label SRV, got rcode %d", code)
	}
}

func TestHandler_SRVRecord_NoPrefix(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)

	req := test.Case{
		Qname: "open-webui.tcp.default.svc.desaules.in.",
		Qtype: dns.TypeSRV,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	code, _ := h.ServeDNS(context.Background(), rec, req)

	if code != dns.RcodeNameError {
		t.Errorf("expected NXDOMAIN for SRV without _ prefix, got rcode %d", code)
	}
}

func TestHandler_SRVRecord_NonExistentService(t *testing.T) {
	store := NewStore()
	populateStore(store)
	h := setupTestHandler(store)

	req := test.Case{
		Qname: "_nonexistent._tcp.default.svc.desaules.in.",
		Qtype: dns.TypeSRV,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	code, _ := h.ServeDNS(context.Background(), rec, req)

	if code != dns.RcodeNameError {
		t.Errorf("expected NXDOMAIN for non-existent SRV service, got rcode %d", code)
	}
}

func TestHandler_SRVRecord_InvalidTarget(t *testing.T) {
	store := NewStore()
	if err := store.Register("bad-srv", "default", &Instance{
		ID: "a b", Address: "10.0.0.1", Port: 8080, Source: "test",
	}); err != nil {
		t.Fatal(err)
	}
	h := setupTestHandler(store)

	req := test.Case{
		Qname: "_bad-srv._tcp.default.svc.desaules.in.",
		Qtype: dns.TypeSRV,
	}.Msg()

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	code, _ := h.ServeDNS(context.Background(), rec, req)

	if code != dns.RcodeNameError {
		t.Errorf("expected NXDOMAIN for invalid SRV target, got rcode %d", code)
	}
}
