package gslb

import (
	"fmt"
	"os/exec"
	"time"
)

type CustomHealthCheck struct {
	Script  string        `yaml:"script"`
	Timeout time.Duration `yaml:"timeout"`
}

func (c *CustomHealthCheck) SetDefault() {
	if c.Timeout == 0 {
		c.Timeout = 5 * time.Second
	}
}

func (c *CustomHealthCheck) PerformCheck(backend *Backend, fqdn string, maxRetries int) bool {
	c.SetDefault()
	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command("/bin/sh", "-c", c.Script)
		cmd.Env = append(cmd.Env, fmt.Sprintf("BACKEND_ADDRESS=%s", backend.Address))
		cmd.Env = append(cmd.Env, fmt.Sprintf("BACKEND_FQDN=%s", fqdn))
		cmd.Env = append(cmd.Env, fmt.Sprintf("BACKEND_PRIORITY=%d", backend.Priority))
		cmd.Env = append(cmd.Env, fmt.Sprintf("BACKEND_ENABLE=%t", backend.Enable))

		err := runWithTimeout(cmd, c.Timeout)
		if err == nil {
			return true
		}
	}
	return false
}

func (c *CustomHealthCheck) GetType() string {
	return "custom"
}

func (c *CustomHealthCheck) Equals(other GenericHealthCheck) bool {
	otherC, ok := other.(*CustomHealthCheck)
	if !ok {
		return false
	}
	return c.Script == otherC.Script && c.Timeout == otherC.Timeout
}

func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return fmt.Errorf("custom healthcheck timeout after %s", timeout)
	}
}
