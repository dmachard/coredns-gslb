package gslb

import (
	"context"
	"net"
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
