package gslb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"gopkg.in/fsnotify.v1"
	"gopkg.in/yaml.v3"
)

// init registers this plugin.
func init() { plugin.Register("gslb", setup) }

// setup is the function that gets called when the config parser see the token "gslb".
func setup(c *caddy.Controller) error {
	config := dnsserver.GetConfig(c)

	// Create a GSLB instance with empty domains and backends
	g := &GSLB{
		Zones:                 make(map[string]string),
		Records:               make(map[string]*Record),
		MaxStaggerStart:       "60s",   // Total time to start all records over time, in seconds
		BatchSizeStart:        100,     // Number of record per group (batch)
		ResolutionIdleTimeout: "3600s", // Max time before to slow down health check
		UseEDNSCSubnet:        false,   // Default: disabled
	}

	for c.Next() {
		if c.Val() == "gslb" {
			// yaml file [zones...]
			if !c.NextArg() {
				return c.ArgErr()
			}
			fileName := c.Val()

			origins := plugin.OriginsFromArgsOrServerBlock(c.RemainingArgs(), c.ServerBlockKeys)
			if !filepath.IsAbs(fileName) && config.Root != "" {
				fileName = filepath.Join(config.Root, fileName)
			}

			// Parse additional options
			for c.NextBlock() {
				switch c.Val() {
				case "use_edns_csubnet":
					if c.NextArg() {
						return c.ArgErr()
					}
					g.UseEDNSCSubnet = true
				case "max_stagger_start":
					if !c.NextArg() {
						return c.ArgErr()
					}
					// Validate duration format for max_stagger_start
					_, err := time.ParseDuration(c.Val())
					if err != nil {
						return fmt.Errorf("invalid value for max_stagger_start, expected duration format: %v", c.Val())
					}
					g.MaxStaggerStart = c.Val()
				case "batch_size_start":
					if !c.NextArg() {
						return c.ArgErr()
					}
					size, err := strconv.Atoi(c.Val())
					if err != nil || size <= 0 {
						return fmt.Errorf("invalid value for batch_size_start: %v", c.Val())
					}
					g.BatchSizeStart = size
				case "resolution_idle_timeout":
					if !c.NextArg() {
						return c.ArgErr()
					}
					// Validate duration format for resolution_idle_timeout
					_, err := time.ParseDuration(c.Val())
					if err != nil {
						return fmt.Errorf("invalid value for resolution_idle_timeout, expected duration format: %v", c.Val())
					}
					g.ResolutionIdleTimeout = c.Val()
				default:
					return c.Errf("unknown option for gslb: %s", c.Val())
				}
			}

			// Read YAML configuration
			if err := loadConfigFile(g, fileName); err != nil {
				return err
			}

			// Read zones

			for i := range origins {
				g.Zones[origins[i]] = fileName
			}

			// Start a goroutine to watch for file modification events
			go startConfigWatcher(g, fileName)
		}
	}

	// Add the Plugin to CoreDNS, so Servers can use it in their plugin chain.
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		g.Next = next
		return g
	})

	// Initialize and load all records
	g.initializeRecords(context.Background())

	// All OK, return a nil error.
	return nil
}

// StartConfigWatcher starts watching the configuration file for changes
func startConfigWatcher(g *GSLB, filePath string) error {
	// Create a new file system watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Add the config file to the watcher
	if err := watcher.Add(filePath); err != nil {
		return fmt.Errorf("failed to add file to watcher: %v", err)
	}

	// Channel for delayed reloads
	var reloadTimer *time.Timer

	// Listen for file system events
	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				// If a timer already exists, cancel it before setting a new one
				if reloadTimer != nil {
					reloadTimer.Stop()
				}

				// Set a new timer to reload the configuration after 500ms
				reloadTimer = time.AfterFunc(500*time.Millisecond, func() {
					// Reload the configuration
					log.Debugf("configuration file modified: %s", filePath)
					if err := reloadConfig(g, filePath); err != nil {
						log.Errorf("failed to reload config: %v", err)
					} else {
						log.Debug("configuration reloaded successfully.")
					}
				})
			}
		case err := <-watcher.Errors:
			if err != nil {
				log.Errorf("Error in file watcher: %v", err)
			}
		}
	}
}

// loadConfigFile loads and parses the YAML configuration file.
func loadConfigFile(g *GSLB, fileName string) error {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("failed to read YAML configuration: %v", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("failed to read YAML configuration: file empty")
	}
	if err := yaml.Unmarshal(data, g); err != nil {
		return fmt.Errorf("failed to parse YAML configuration: %v", err)
	}
	return nil
}

// ReloadConfig updates the GSLB configuration dynamically
func reloadConfig(g *GSLB, filePath string) error {
	// Ensure the Records map is initialized
	if g.Records == nil {
		g.Records = make(map[string]*Record)
	}

	g.Mutex.Lock()
	defer g.Mutex.Unlock()

	// Read YAML configuration
	newGSLB := &GSLB{}
	if err := loadConfigFile(newGSLB, filePath); err != nil {
		return err
	}

	// Update GSLB
	g.updateRecords(context.Background(), newGSLB)

	return nil
}
