package gslb

import (
	"context"
	"net"
	"testing"

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
