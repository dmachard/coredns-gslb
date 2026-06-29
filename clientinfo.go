package gslb

import (
	"context"
	"net"
	"strings"

	"github.com/miekg/dns"
)

type clientCtxKey struct{}

type ClientInfo struct {
	IP        net.IP
	PrefixLen uint8
}

func WithClientInfo(ctx context.Context, ip net.IP, prefix uint8) context.Context {
	return context.WithValue(ctx, clientCtxKey{}, &ClientInfo{IP: ip, PrefixLen: prefix})
}

func GetClientInfo(ctx context.Context) *ClientInfo {
	val := ctx.Value(clientCtxKey{})
	if info, ok := val.(*ClientInfo); ok {
		return info
	}
	return nil
}

type ecsResponseWriter struct {
	dns.ResponseWriter
	g      *GSLB
	reqMsg *dns.Msg
}

func (w *ecsResponseWriter) WriteMsg(res *dns.Msg) error {
	if w.reqMsg != nil && len(w.reqMsg.Question) > 0 {
		domain := strings.ToLower(dns.Fqdn(strings.TrimSuffix(w.reqMsg.Question[0].Name, ".")))
		w.g.decorateWithECS(w.reqMsg, res, domain, true)
	}
	return w.ResponseWriter.WriteMsg(res)
}
