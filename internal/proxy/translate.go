package proxy

import "github.com/mescon/muximux/v3/internal/config"

// ConfigGatewaySitesToProxy copies the config-package site list into
// the proxy package's mirror type. The two structs are deliberately
// duplicated to keep the proxy package free of a config import on the
// hot path; this helper is the single bridge between them.
//
// Lives in the proxy package (not handlers) so any caller that needs
// to translate -- the gateway handler, the discovery poller, the
// server bootstrap -- can call it without creating a handlers
// dependency. The proxy package already imports config (for
// ServerConfig and others), so this function adds no new edges to
// the dependency graph.
func ConfigGatewaySitesToProxy(sites []config.GatewaySite) []GatewaySite {
	if len(sites) == 0 {
		return nil
	}
	out := make([]GatewaySite, len(sites))
	for i := range sites {
		s := &sites[i]
		out[i] = GatewaySite{
			Domain:               s.Domain,
			BackendURL:           s.BackendURL,
			TLS:                  string(s.TLS),
			TLSCert:              s.TLSCert,
			TLSKey:               s.TLSKey,
			StripFrameBlockers:   s.StripFrameBlockers,
			Streaming:            s.Streaming,
			BackendSkipTLSVerify: s.BackendSkipTLSVerify,
			ProxyHeaders:         s.ProxyHeaders,
			ForwardedHeaders:     s.ForwardedHeaders,
			RequireAuth:          s.RequireAuth,
		}
	}
	return out
}
