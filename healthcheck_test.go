package gslb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test error handling for unsupported health check types
func TestToSpecificHealthCheck_Unsupported(t *testing.T) {
	// Test for a health check with an unknown type
	hc := &HealthCheck{
		Type:   "unsupported_type",
		Params: map[string]interface{}{},
	}

	// Ensure the conversion fails
	_, err := hc.ToSpecificHealthCheck()
	assert.Error(t, err)
	assert.Equal(t, "unsupported healthcheck type: unsupported_type", err.Error())
}

// Test that all known healthcheck types are handled in ToSpecificHealthCheck
func TestToSpecificHealthCheck_AllTypesHandled(t *testing.T) {
	types := []string{"http", "icmp", "tcp", "mysql", "grpc", "lua"}
	for _, typ := range types {
		hc := &HealthCheck{
			Type:   typ,
			Params: map[string]interface{}{},
		}
		_, err := hc.ToSpecificHealthCheck()
		assert.NoErrorf(t, err, "Type '%s' should be handled in ToSpecificHealthCheck", typ)
	}
}

func TestToSpecificHealthCheck_AllKnownTypes(t *testing.T) {
	types := []string{"http", "icmp", "tcp", "mysql", "grpc", "lua"}
	for _, typ := range types {
		hc := &HealthCheck{
			Type:   typ,
			Params: map[string]interface{}{},
		}
		_, err := hc.ToSpecificHealthCheck()
		if err != nil {
			t.Errorf("Type '%s' should be handled in ToSpecificHealthCheck, got error: %v", typ, err)
		}
	}
}

func TestHealthCheck_GetType(t *testing.T) {
	// Test HTTP health check
	httpHC := &HTTPHealthCheck{}
	assert.Equal(t, "http/0", httpHC.GetType())

	// Test TCP health check
	tcpHC := &TCPHealthCheck{}
	assert.Equal(t, "tcp/0", tcpHC.GetType())

	// Test ICMP health check
	icmpHC := &ICMPHealthCheck{}
	assert.Equal(t, "icmp", icmpHC.GetType())

	// Test MySQL health check
	mysqlHC := &MySQLHealthCheck{}
	assert.Equal(t, "mysql/0", mysqlHC.GetType())

	// Test gRPC health check
	grpcHC := &GRPCHealthCheck{}
	assert.Equal(t, "grpc", grpcHC.GetType())

	// Test Lua health check
	luaHC := &LuaHealthCheck{}
	assert.Equal(t, "lua", luaHC.GetType())
}

func TestHealthChecksEqual(t *testing.T) {
	// Test with empty slices
	assert.True(t, healthChecksEqual([]GenericHealthCheck{}, []GenericHealthCheck{}))

	// Test with different lengths
	hc1 := []GenericHealthCheck{&MockHealthCheck{}}
	hc2 := []GenericHealthCheck{}
	assert.False(t, healthChecksEqual(hc1, hc2))

	// Test with same health checks
	hc3 := []GenericHealthCheck{&MockHealthCheck{}}
	hc4 := []GenericHealthCheck{&MockHealthCheck{}}
	assert.True(t, healthChecksEqual(hc3, hc4))

	// Test with different health checks
	httpHC := &HTTPHealthCheck{}
	httpHC.SetDefault()
	tcpHC := &TCPHealthCheck{}
	tcpHC.SetDefault()

	hc5 := []GenericHealthCheck{httpHC}
	hc6 := []GenericHealthCheck{tcpHC}
	assert.False(t, healthChecksEqual(hc5, hc6))

	// Test with multiple health checks
	hc7 := []GenericHealthCheck{httpHC, tcpHC}
	hc8 := []GenericHealthCheck{httpHC, tcpHC}
	assert.True(t, healthChecksEqual(hc7, hc8))
}

func TestHealthCheck_Equals(t *testing.T) {
	// Test HTTP health check Equals
	httpHC1 := &HTTPHealthCheck{}
	httpHC1.SetDefault()
	httpHC2 := &HTTPHealthCheck{}
	httpHC2.SetDefault()
	assert.True(t, httpHC1.Equals(httpHC2))

	// Test TCP health check Equals
	tcpHC1 := &TCPHealthCheck{}
	tcpHC1.SetDefault()
	tcpHC2 := &TCPHealthCheck{}
	tcpHC2.SetDefault()
	assert.True(t, tcpHC1.Equals(tcpHC2))

	// Test ICMP health check Equals
	icmpHC1 := &ICMPHealthCheck{}
	icmpHC1.SetDefault()
	icmpHC2 := &ICMPHealthCheck{}
	icmpHC2.SetDefault()
	assert.True(t, icmpHC1.Equals(icmpHC2))

	// Test MySQL health check Equals
	mysqlHC1 := &MySQLHealthCheck{}
	mysqlHC1.SetDefault()
	mysqlHC2 := &MySQLHealthCheck{}
	mysqlHC2.SetDefault()
	assert.True(t, mysqlHC1.Equals(mysqlHC2))

	// Test gRPC health check Equals
	grpcHC1 := &GRPCHealthCheck{}
	grpcHC1.SetDefault()
	grpcHC2 := &GRPCHealthCheck{}
	grpcHC2.SetDefault()
	assert.True(t, grpcHC1.Equals(grpcHC2))

	// Test Lua health check Equals
	luaHC1 := &LuaHealthCheck{}
	luaHC1.SetDefault()
	luaHC2 := &LuaHealthCheck{}
	luaHC2.SetDefault()
	assert.True(t, luaHC1.Equals(luaHC2))

	// Test different types are not equal
	assert.False(t, httpHC1.Equals(tcpHC1))
	assert.False(t, tcpHC1.Equals(icmpHC1))
	assert.False(t, icmpHC1.Equals(mysqlHC1))
}

func TestHealthCheck_PerformCheck(t *testing.T) {
	// Test gRPC health check
	grpcHC := &GRPCHealthCheck{}
	grpcHC.SetDefault()
	backend := &Backend{
		Address: "localhost",
		Enable:  true,
	}

	// Test that PerformCheck doesn't panic (it will fail but shouldn't crash)
	assert.NotPanics(t, func() {
		result := grpcHC.PerformCheck(backend, "test.example.com", 1)
		// Should return false since no gRPC server is running
		assert.False(t, result)
	})

	// Test ICMP health check
	icmpHC := &ICMPHealthCheck{}
	icmpHC.SetDefault()

	assert.NotPanics(t, func() {
		result := icmpHC.PerformCheck(backend, "test.example.com", 1)
		// Should return false since ICMP requires privileges
		assert.False(t, result)
	})

	// Test MySQL health check
	mysqlHC := &MySQLHealthCheck{}
	mysqlHC.SetDefault()

	assert.NotPanics(t, func() {
		result := mysqlHC.PerformCheck(backend, "test.example.com", 1)
		// Should return false since no MySQL server is running
		assert.False(t, result)
	})
}
