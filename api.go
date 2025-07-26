package gslb

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const statusHealthy = "healthy"
const statusUnhealthy = "unhealthy"

// checkBasicAuth checks HTTP Basic Auth if configured, returns true if authorized, false otherwise.
func (g *GSLB) checkBasicAuth(w http.ResponseWriter, r *http.Request) bool {
	if g.APIBasicUser != "" && g.APIBasicPass != "" {
		user, pass, ok := r.BasicAuth()
		if !ok || user != g.APIBasicUser || pass != g.APIBasicPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="GSLB API"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return false
		}
	}
	return true
}

// handleBulkSetBackendEnable returns a handler that enables or disables backends in bulk.
func (g *GSLB) handleBulkSetBackendEnable(enable bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.checkBasicAuth(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed. Only POST is supported."})
			return
		}
		var req struct {
			Location      string `json:"location"`
			AddressPrefix string `json:"address_prefix"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
			return
		}
		if req.Location == "" && req.AddressPrefix == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "location or address_prefix required"})
			return
		}
		var allModified []map[string]string
		for _, yamlFile := range g.Zones {
			modified, err := bulkSetBackendEnable(yamlFile, req.Location, req.AddressPrefix, enable)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			allModified = append(allModified, modified...)
		}
		resp := map[string]interface{}{
			"success":  true,
			"backends": allModified,
		}
		if resp["backends"] == nil {
			resp["backends"] = []map[string]string{}
		}
		json.NewEncoder(w).Encode(resp)
	}
}

// handleOverview returns a simplified overview of all records and their backends.
func (g *GSLB) handleOverview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.checkBasicAuth(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed. Only GET is supported."})
			return
		}
		g.Mutex.RLock()
		defer g.Mutex.RUnlock()
		var resp []map[string]interface{}
		for zone, recs := range g.Records {
			for _, rec := range recs {
				rec.mutex.RLock()
				atLeastOneBackendHealthy := false
				var backends []map[string]interface{}
				for _, be := range rec.Backends {
					b := be.(*Backend)
					b.mutex.RLock()
					aliveStr := statusUnhealthy
					if b.Alive && b.Enable {
						aliveStr = statusHealthy
						atLeastOneBackendHealthy = true
					}
					beMap := map[string]interface{}{
						"address":          b.Address,
						"alive":            aliveStr,
						"last_healthcheck": b.LastHealthcheck.Format(time.RFC3339),
					}
					b.mutex.RUnlock()
					backends = append(backends, beMap)
				}
				recMap := map[string]interface{}{
					"zone":   zone,
					"record": rec.Fqdn,
					"status": func() string {
						if atLeastOneBackendHealthy {
							return statusHealthy
						}
						return statusUnhealthy
					}(),
					"backends": backends,
				}
				resp = append(resp, recMap)
				rec.mutex.RUnlock()
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// RegisterAPIHandlers registers all API endpoints to the provided mux.
func (g *GSLB) RegisterAPIHandlers(mux *http.ServeMux) {
	// Handler for /api/overview (GET only, with optional HTTP Basic Auth)
	mux.HandleFunc("/api/overview", g.handleOverview())

	// Handler for bulk disable (POST /api/backends/disable)
	mux.HandleFunc("/api/backends/disable", g.handleBulkSetBackendEnable(false))
	// Handler for bulk enable (POST /api/backends/enable)
	mux.HandleFunc("/api/backends/enable", g.handleBulkSetBackendEnable(true))
}

// bulkSetBackendEnable sets enable=true or false for all backends matching location or addressPrefix in the YAML config file.
// Returns the number of backends modified and any error.
func bulkSetBackendEnable(yamlFile, location, addressPrefix string, enable bool) ([]map[string]string, error) {
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	records, ok := raw["records"].(map[string]interface{})
	if !ok {
		return nil, err
	}
	var modified []map[string]string
	for fqdn, rec := range records {
		recMap, ok := rec.(map[string]interface{})
		if !ok {
			continue
		}
		backends, ok := recMap["backends"].([]interface{})
		if !ok {
			continue
		}
		for _, be := range backends {
			beMap, ok := be.(map[string]interface{})
			if !ok {
				continue
			}
			addr, _ := beMap["address"].(string)
			loc, _ := beMap["location"].(string)
			match := false
			if location != "" && loc == location {
				match = true
			}
			if addressPrefix != "" && strings.HasPrefix(addr, addressPrefix) {
				match = true
			}
			if match {
				beMap["enable"] = enable
				modified = append(modified, map[string]string{
					"fqdn":    fqdn,
					"address": addr,
				})
			}
		}
		recMap["backends"] = backends
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)

	if err := encoder.Encode(raw); err != nil {
		return nil, err
	}
	encoder.Close()

	if err := os.WriteFile(yamlFile, buf.Bytes(), 0644); err != nil {
		return nil, err
	}
	return modified, nil
}
