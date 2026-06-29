package gslb

import (
	"context"
	"net"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

func TestWithAndGetClientInfo(t *testing.T) {
	ctx := context.Background()
	ip := net.ParseIP("192.0.2.1")
	prefix := uint8(24)
	ctxWithInfo := WithClientInfo(ctx, ip, prefix)
	ci := GetClientInfo(ctxWithInfo)
	assert.NotNil(t, ci)
	assert.Equal(t, ip, ci.IP)
	assert.Equal(t, prefix, ci.PrefixLen)

	// Test fallback: no info in context
	empty := GetClientInfo(context.Background())
	assert.Nil(t, empty)
}

func TestEcsResponseWriter(t *testing.T) {
	g := &GSLB{
		UseEDNSCSubnet: true,
	}

	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeA)
	opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	ecs := &dns.EDNS0_SUBNET{
		Code:          dns.EDNS0SUBNET,
		Address:       net.ParseIP("198.51.100.0"),
		SourceNetmask: 24,
		Family:        1,
	}
	opt.Option = append(opt.Option, ecs)
	req.Extra = append(req.Extra, opt)

	mockW := &mockResponseWriter{msg: new(dns.Msg)}

	w := &ecsResponseWriter{
		ResponseWriter: mockW,
		g:              g,
		reqMsg:         req,
	}

	res := new(dns.Msg)
	res.SetReply(req)

	err := w.WriteMsg(res)
	assert.NoError(t, err)

	optRes := mockW.msg.IsEdns0()
	assert.NotNil(t, optRes)
	var foundEcs *dns.EDNS0_SUBNET
	for _, o := range optRes.Option {
		if ecsOpt, ok := o.(*dns.EDNS0_SUBNET); ok {
			foundEcs = ecsOpt
			break
		}
	}
	assert.NotNil(t, foundEcs)
	assert.Equal(t, "198.51.100.0", foundEcs.Address.String())
	assert.Equal(t, uint8(24), foundEcs.SourceNetmask)
}
