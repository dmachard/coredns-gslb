package gslb

import (
	"errors"
	"os"
	"testing"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/stretchr/testify/assert"
)

// requires that it be run with super-user privileges.
func TestICMPHealthCheckPerformCheck(t *testing.T) {
	healthCheck := &ICMPHealthCheck{
		Count:   2,
		Timeout: "5s",
	}

	backend := &Backend{
		Address: "127.0.0.1", // Ping localhost
	}

	fqdn := "test.localhost"

	result := healthCheck.PerformCheck(backend, fqdn, 1)

	// Since raw ICMP sockets require privileges (root or setcap),
	// we only assert success when running as root.
	if os.Getuid() == 0 {
		assert.True(t, result, "ICMP health check should succeed for localhost when run as root")
	} else {
		t.Logf("ICMP health check returned %v (non-root run, skipping strict assertion)", result)
	}
}

type mockPinger struct {
	runErr     error
	stats      *probing.Statistics
	privileged bool
}

func (m *mockPinger) Run() error {
	return m.runErr
}

func (m *mockPinger) Statistics() *probing.Statistics {
	return m.stats
}

func (m *mockPinger) SetPrivileged(privileged bool) {
	m.privileged = privileged
}

func TestICMPHealthCheck_Unit(t *testing.T) {
	// Save the original createPinger
	orgCreatePinger := createPinger
	defer func() {
		createPinger = orgCreatePinger
	}()

	t.Run("invalid timeout format", func(t *testing.T) {
		h := &ICMPHealthCheck{
			Count:   2,
			Timeout: "invalid-timeout",
		}
		backend := &Backend{Address: "127.0.0.1"}
		res := h.PerformCheck(backend, "test.local.", 1)
		assert.False(t, res)
	})

	t.Run("createPinger fails on all retries", func(t *testing.T) {
		createPinger = func(address string, count int, timeout time.Duration) (Pinger, error) {
			return nil, errors.New("pinger init error")
		}

		h := &ICMPHealthCheck{
			Count:   2,
			Timeout: "1s",
		}
		backend := &Backend{Address: "127.0.0.1"}
		res := h.PerformCheck(backend, "test.local.", 2)
		assert.False(t, res)
	})

	t.Run("pinger Run fails on all retries", func(t *testing.T) {
		createPinger = func(address string, count int, timeout time.Duration) (Pinger, error) {
			return &mockPinger{runErr: errors.New("run error")}, nil
		}

		h := &ICMPHealthCheck{
			Count:   2,
			Timeout: "1s",
		}
		backend := &Backend{Address: "127.0.0.1"}
		res := h.PerformCheck(backend, "test.local.", 2)
		assert.False(t, res)
	})

	t.Run("pinger succeeds but 0 packets received", func(t *testing.T) {
		createPinger = func(address string, count int, timeout time.Duration) (Pinger, error) {
			return &mockPinger{
				stats: &probing.Statistics{
					PacketsRecv: 0,
					PacketsSent: 3,
				},
			}, nil
		}

		h := &ICMPHealthCheck{
			Count:   3,
			Timeout: "1s",
		}
		backend := &Backend{Address: "127.0.0.1"}
		res := h.PerformCheck(backend, "test.local.", 1)
		assert.False(t, res)
	})

	t.Run("pinger succeeds on second try", func(t *testing.T) {
		tries := 0
		createPinger = func(address string, count int, timeout time.Duration) (Pinger, error) {
			tries++
			if tries == 1 {
				return nil, errors.New("temporary init error")
			}
			return &mockPinger{
				stats: &probing.Statistics{
					PacketsRecv: 2,
					PacketsSent: 3,
				},
			}, nil
		}

		h := &ICMPHealthCheck{
			Count:   3,
			Timeout: "1s",
		}
		backend := &Backend{Address: "127.0.0.1"}
		res := h.PerformCheck(backend, "test.local.", 2)
		assert.True(t, res)
		assert.Equal(t, 2, tries)
	})

	t.Run("Equals", func(t *testing.T) {
		h1 := &ICMPHealthCheck{Count: 3, Timeout: "5s"}
		h2 := &ICMPHealthCheck{Count: 3, Timeout: "5s"}
		h3 := &ICMPHealthCheck{Count: 4, Timeout: "5s"}
		h4 := &ICMPHealthCheck{Count: 3, Timeout: "10s"}

		assert.True(t, h1.Equals(h2))
		assert.False(t, h1.Equals(h3))
		assert.False(t, h1.Equals(h4))
		assert.False(t, h1.Equals(&HTTPHealthCheck{}))
	})
}

func TestRealPinger_Coverage(t *testing.T) {
	p, err := createPinger("127.0.0.1", 1, time.Millisecond)
	if err == nil && p != nil {
		p.SetPrivileged(false)
		_ = p.Run()
		_ = p.Statistics()
	}
}
