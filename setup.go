package gslb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/oschwald/geoip2-golang"
	"gopkg.in/fsnotify.v1"
	"gopkg.in/yaml.v3"
)

// init registers this plugin.
func init() { plugin.Register("gslb", setup) }

// Version of the GSLB plugin, set at build time
var Version = "dev"

// setup is the function that gets called when the config parser see the token "gslb".
func setup(c *caddy.Controller) error {
	RegisterMetrics()
	SetVersionInfo(Version)

	config := dnsserver.GetConfig(c)

	g := &GSLB{
		Zones:                     make(map[string]string),
		Records:                   make(map[string]map[string]*Record),
		LocationMap:               make(map[string]string),
		MaxStaggerStart:           "60s",
		BatchSizeStart:            100,
		ResolutionIdleTimeout:     "3600s",
		UseEDNSCSubnet:            false,
		HealthcheckIdleMultiplier: 10,
		APIEnable:                 true,
		APIListenAddr:             "0.0.0.0",
		APIListenPort:             "8080",
	}

	zoneFiles := make(map[string]string)

	for c.Next() {
		if c.Val() == "gslb" {
			locationMapPath := ""
			for c.NextBlock() {
				switch c.Val() {
				case "zone":
					if !c.NextArg() {
						return c.ArgErr()
					}
					zone := c.Val()
					if !c.NextArg() {
						return c.ArgErr()
					}
					file := c.Val()
					if !filepath.IsAbs(file) && config.Root != "" {
						file = filepath.Join(config.Root, file)
					}
					zoneNorm := strings.ToLower(strings.TrimSuffix(zone, ".")) + "."
					zoneFiles[zoneNorm] = file

					g.Zones[zoneNorm] = file
					go startConfigWatcher(g, file)
				case "use_edns_csubnet":
					if c.NextArg() {
						return c.ArgErr()
					}
					g.UseEDNSCSubnet = true
				case "max_stagger_start":
					if !c.NextArg() {
						return c.ArgErr()
					}
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
					_, err := time.ParseDuration(c.Val())
					if err != nil {
						return fmt.Errorf("invalid value for resolution_idle_timeout, expected duration format: %v", c.Val())
					}
					g.ResolutionIdleTimeout = c.Val()
				case "geoip_custom":
					if !c.NextArg() {
						return c.ArgErr()
					}
					locationMapPath = c.Val()
					if err := g.loadCustomLocationsMap(locationMapPath); err != nil {
						return fmt.Errorf("failed to load location map: %w", err)
					}
				case "geoip_maxmind":
					if c.NextArg() {
						typeArg := c.Val()
						if !c.NextArg() {
							return c.ArgErr()
						}
						pathArg := c.Val()
						switch typeArg {
						case "country_db":
							countryDB, err := geoip2.Open(pathArg)
							if err != nil {
								return fmt.Errorf("failed to open country MaxMind DB: %w", err)
							}
							g.GeoIPCountryDB = countryDB
						case "city_db":
							cityDB, err := geoip2.Open(pathArg)
							if err != nil {
								return fmt.Errorf("failed to open city MaxMind DB: %w", err)
							}
							g.GeoIPCityDB = cityDB
						case "asn_db":
							asnDB, err := geoip2.Open(pathArg)
							if err != nil {
								return fmt.Errorf("failed to open ASN MaxMind DB: %w", err)
							}
							g.GeoIPASNDB = asnDB
						default:
							return c.Errf("unknown geoip_maxmind type: %s", typeArg)
						}
					}
				case "healthcheck_idle_multiplier":
					if !c.NextArg() {
						return c.ArgErr()
					}
					mult, err := strconv.Atoi(c.Val())
					if err != nil || mult < 1 {
						return fmt.Errorf("invalid value for healthcheck_idle_multiplier: %v", c.Val())
					}
					g.HealthcheckIdleMultiplier = mult
				case "api_enable":
					if !c.NextArg() {
						return c.ArgErr()
					}
					val := c.Val()
					if val == "false" || val == "0" {
						g.APIEnable = false
					} else {
						g.APIEnable = true
					}
				case "api_tls_cert":
					if !c.NextArg() {
						return c.ArgErr()
					}
					g.APICertPath = c.Val()
				case "api_tls_key":
					if !c.NextArg() {
						return c.ArgErr()
					}
					g.APIKeyPath = c.Val()
				case "api_listen_addr":
					if !c.NextArg() {
						return c.ArgErr()
					}
					g.APIListenAddr = c.Val()
				case "api_listen_port":
					if !c.NextArg() {
						return c.ArgErr()
					}
					g.APIListenPort = c.Val()
				case "api_basic_user":
					if !c.NextArg() {
						return c.ArgErr()
					}
					g.APIBasicUser = c.Val()
				case "api_basic_pass":
					if !c.NextArg() {
						return c.ArgErr()
					}
					g.APIBasicPass = c.Val()
				case "healthcheck_profiles":
					if !c.NextArg() {
						return c.ArgErr()
					}
					globalProfilesPath := c.Val()
					data, err := os.ReadFile(globalProfilesPath)
					if err != nil {
						return fmt.Errorf("failed to read global healthcheck_profiles: %w", err)
					}
					var tmp struct {
						HealthcheckProfiles map[string]*HealthCheck `yaml:"healthcheck_profiles"`
					}
					if err := yaml.Unmarshal(data, &tmp); err != nil {
						return fmt.Errorf("failed to parse global healthcheck_profiles: %w", err)
					}
					GlobalHealthcheckProfiles = tmp.HealthcheckProfiles
				case "disable_txt":
					if c.NextArg() {
						return c.ArgErr()
					}
					g.DisableTXT = true
				default:
					return c.Errf("unknown option for gslb: %s", c.Val())
				}
			}
			if len(zoneFiles) == 0 {
				return c.Errf("at least one 'zone' directive is required in gslb block")
			}
			if locationMapPath != "" {
				go watchCustomLocationMap(g, locationMapPath)
			}
			if g.APIEnable {
				go g.ServeAPI()
			}
		}
	}

	// Add the Plugin to CoreDNS, so Servers can use it in their plugin chain.
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		g.Next = next
		return g
	})

	// Initialize and load all records
	g.initializeRecordsFromFiles(context.Background(), zoneFiles)

	// All OK, return a nil error.
	return nil
}

// StartConfigWatcher starts watching the configuration file for changes
func startConfigWatcher(g *GSLB, filePath string) error {
	log.Debugf("Starting config watcher for %s", filePath)
	// Create a new file system watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	// Add the config file to the watcher
	if err := watcher.Add(filePath); err != nil {
		return fmt.Errorf("failed to add file to watcher: %w", err)
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
					log.Infof("Configuration file modified: %s", filePath)
					zone := findZoneByFile(g, filePath)
					if zone == "" {
						log.Errorf("Zone not found for file: %s", filePath)
						return
					}
					if err := reloadConfig(g, filePath, zone); err != nil {
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

// ReloadConfig updates the GSLB configuration dynamically
func reloadConfig(g *GSLB, filePath string, zone string) error {
	log.Infof("Reloading config from %s", filePath)

	// Ensure the Records map is initialized
	if g.Records == nil {
		g.Records = make(map[string]map[string]*Record)
	}

	g.Mutex.Lock()
	defer g.Mutex.Unlock()

	// Read YAML configuration
	newGSLB := &GSLB{}
	if err := loadConfigFile(newGSLB, filePath, zone); err != nil {
		IncConfigReloads("failure")
		return err
	}

	// Update GSLB
	g.updateRecords(context.Background(), newGSLB)
	IncConfigReloads("success")
	return nil
}

// Add a dedicated watcher for the custom location map
func watchCustomLocationMap(g *GSLB, locationMapPath string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Errorf("failed to create watcher for custom location map: %v", err)
		return
	}
	defer watcher.Close()

	if err := watcher.Add(locationMapPath); err != nil {
		log.Errorf("failed to add custom location map to watcher: %v", err)
		return
	}

	var reloadTimer *time.Timer

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				if reloadTimer != nil {
					reloadTimer.Stop()
				}
				reloadTimer = time.AfterFunc(500*time.Millisecond, func() {
					log.Debugf("custom location map file modified: %s", locationMapPath)
					if err := g.loadCustomLocationsMap(locationMapPath); err != nil {
						log.Errorf("failed to reload custom location map: %v", err)
					} else {
						log.Debug("custom location map reloaded successfully.")
					}
				})
			}
		case err := <-watcher.Errors:
			if err != nil {
				log.Errorf("Error in custom location map watcher: %v", err)
			}
		}
	}
}

// Ajoute une fonction utilitaire pour retrouver la zone à partir du fichier
func findZoneByFile(g *GSLB, filePath string) string {
	for zone, file := range g.Zones {
		if file == filePath {
			return zone
		}
	}
	return ""
}
