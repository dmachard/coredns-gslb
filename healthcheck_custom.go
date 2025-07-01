package gslb

import (
	"context"
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
	typeStr := c.GetType()
	address := backend.Address
	start := time.Now()
	result := false
	log.Debugf("[custom] Starting custom healthcheck for backend: %s (script: %s, timeout: %s)", address, c.Script, c.Timeout)
	defer func() {
		ObserveHealthcheck(typeStr, address, start, result)
		log.Debugf("[custom] Custom healthcheck for backend %s result: %v", address, result)
	}()

	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", c.Script)
		cmd.Env = append(cmd.Env, fmt.Sprintf("BACKEND_ADDRESS=%s", backend.Address))
		cmd.Env = append(cmd.Env, fmt.Sprintf("BACKEND_FQDN=%s", fqdn))
		cmd.Env = append(cmd.Env, fmt.Sprintf("BACKEND_PRIORITY=%d", backend.Priority))
		cmd.Env = append(cmd.Env, fmt.Sprintf("BACKEND_ENABLE=%t", backend.Enable))

		log.Debugf("[custom] Executing script for backend %s (attempt %d/%d)", address, i+1, maxRetries)
		err := cmd.Run()
		if err == nil {
			log.Debugf("[custom] Script succeeded for backend %s", address)
			result = true
			return true
		}
		if ctx.Err() == context.DeadlineExceeded {
			log.Debugf("[custom] Script timeout for backend %s", address)
			return false
		}
		log.Debugf("[custom] Script failed for backend %s: %v", address, err)
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
