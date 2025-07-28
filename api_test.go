package gslb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"encoding/base64"
	"os"
	"strings"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPIOverviewEndpoint(t *testing.T) {
	g := &GSLB{
		Records: make(map[string]map[string]*Record),
	}
	rec := &Record{
		Fqdn:           "test.example.com.",
		Mode:           "failover",
		Owner:          "owner",
		Description:    "desc",
		RecordTTL:      30,
		ScrapeInterval: "10s",
		ScrapeRetries:  2,
		ScrapeTimeout:  "5s",
	}
	backend := &Backend{
		Address:         "1.2.3.4",
		Priority:        1,
		Enable:          true,
		Alive:           true,
		Description:     "backend-desc",
		Location:        "edge-eu",
		LastHealthcheck: time.Date(2025, 7, 21, 13, 3, 29, 0, time.UTC),
	}
	rec.Backends = []BackendInterface{backend}
	g.Records["test."] = map[string]*Record{rec.Fqdn: rec}

	mux := http.NewServeMux()
	g.RegisterAPIHandlers(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/overview")
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	var apiResp map[string][]map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&apiResp)
	assert.NoError(t, err)
	zoneRecords, ok := apiResp["test."]
	assert.True(t, ok, "zone 'test.' should be present in response")
	assert.Len(t, zoneRecords, 1)
	recResp := zoneRecords[0]
	assert.Equal(t, "test.example.com.", recResp["record"])
	assert.Equal(t, "healthy", recResp["status"])
	backends, ok := recResp["backends"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, backends, 1)
	be := backends[0].(map[string]interface{})
	assert.Equal(t, "1.2.3.4", be["address"])
	assert.Equal(t, "healthy", be["alive"])
	assert.Equal(t, "2025-07-21T13:03:29Z", be["last_healthcheck"])
}

func TestAPIOverviewZoneEndpoint(t *testing.T) {
	g := &GSLB{
		Records: make(map[string]map[string]*Record),
	}
	rec1 := &Record{
		Fqdn: "webapp1.zone1.example.com.",
	}
	rec2 := &Record{
		Fqdn: "webapp2.zone2.example.com.",
	}
	g.Records["zone1.example.com."] = map[string]*Record{rec1.Fqdn: rec1}
	g.Records["zone2.example.com."] = map[string]*Record{rec2.Fqdn: rec2}

	mux := http.NewServeMux()
	g.RegisterAPIHandlers(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Test zone1
	resp, err := http.Get(ts.URL + "/api/overview/zone1.example.com.")
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	var records []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&records)
	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, "webapp1.zone1.example.com.", records[0]["record"])

	// Test zone2
	resp2, err := http.Get(ts.URL + "/api/overview/zone2.example.com.")
	assert.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, 200, resp2.StatusCode)
	var records2 []map[string]interface{}
	err = json.NewDecoder(resp2.Body).Decode(&records2)
	assert.NoError(t, err)
	assert.Len(t, records2, 1)
	assert.Equal(t, "webapp2.zone2.example.com.", records2[0]["record"])

	// Test zone not found
	resp3, err := http.Get(ts.URL + "/api/overview/unknownzone.com.")
	assert.NoError(t, err)
	defer resp3.Body.Close()
	assert.Equal(t, 404, resp3.StatusCode)
	var errResp map[string]interface{}
	err = json.NewDecoder(resp3.Body).Decode(&errResp)
	assert.NoError(t, err)
	assert.Contains(t, errResp["error"], "Zone not found")
}

func TestAPIDisableBackendsEndpoint(t *testing.T) {
	tempYaml := `records:
  test.example.com.:
    backends:
      - address: "1.2.3.4"
        enable: true
        location: "dc-eu"
      - address: "1.2.3.5"
        enable: true
        location: "dc-us"
      - address: "10.0.0.1"
        enable: true
        location: "dc-eu"
      - address: "172.16.0.99"
        enable: true
        location: "dc-eu"
`
	f, err := os.CreateTemp("", "gslb_test_*.yml")
	assert.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.Write([]byte(tempYaml))
	assert.NoError(t, err)
	f.Close()

	g := &GSLB{
		Zones: map[string]string{"test": f.Name()},
	}
	mux := http.NewServeMux()
	g.RegisterAPIHandlers(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Désactivation par location
	resp, err := http.Post(ts.URL+"/api/backends/disable", "application/json", strings.NewReader(`{"location":"dc-eu"}`))
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	assert.NoError(t, err)
	assert.True(t, body["success"].(bool))
	backends, ok := body["backends"]
	assert.True(t, ok, "backends field should be present")
	beList, isList := backends.([]interface{})
	assert.True(t, isList, "backends should be a list (even if empty)")
	assert.Len(t, beList, 3)
	expectedBackends := []map[string]string{
		{"record": "test.example.com.", "address": "1.2.3.4"},
		{"record": "test.example.com.", "address": "10.0.0.1"},
		{"record": "test.example.com.", "address": "172.16.0.99"},
	}
	for _, expected := range expectedBackends {
		found := false
		for _, actual := range beList {
			be := actual.(map[string]interface{})
			if be["record"] == expected["record"] && be["address"] == expected["address"] {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected backend %+v not found in response", expected)
	}

	// Désactivation par préfixe d'IP
	_ = os.WriteFile(f.Name(), []byte(tempYaml), 0644) // reset file
	resp2, err := http.Post(ts.URL+"/api/backends/disable", "application/json", strings.NewReader(`{"address_prefix":"1.2.3."}`))
	assert.NoError(t, err)
	defer resp2.Body.Close()
	var body3 map[string]interface{}
	err = json.NewDecoder(resp2.Body).Decode(&body3)
	assert.NoError(t, err)
	assert.True(t, body3["success"].(bool))
	backends2, ok2 := body3["backends"]
	assert.True(t, ok2, "backends field should be present")
	beList2, isList2 := backends2.([]interface{})
	assert.True(t, isList2, "backends should be a list (even if empty)")
	assert.Len(t, beList2, 2)
	expectedBackends2 := []map[string]string{
		{"record": "test.example.com.", "address": "1.2.3.4"},
		{"record": "test.example.com.", "address": "1.2.3.5"},
	}
	for _, expected := range expectedBackends2 {
		found := false
		for _, actual := range beList2 {
			be := actual.(map[string]interface{})
			if be["record"] == expected["record"] && be["address"] == expected["address"] {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected backend %+v not found in response", expected)
	}

	// Cas d'erreur : mauvais body
	resp3, err := http.Post(ts.URL+"/api/backends/disable", "application/json", strings.NewReader(`{}`))
	assert.NoError(t, err)
	defer resp3.Body.Close()
	assert.Equal(t, 400, resp3.StatusCode)

	// Cas d'erreur : mauvaise méthode
	req, _ := http.NewRequest("GET", ts.URL+"/api/backends/disable", nil)
	resp4, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp4.Body.Close()
	assert.Equal(t, 405, resp4.StatusCode)
}

func TestAPIEnableBackendsEndpoint(t *testing.T) {
	// Prepare a temporary YAML file with various backends
	tempYaml := `records:
  test.example.com.:
    backends:
      - address: "1.2.3.4"
        enable: false
        location: "dc-eu"
      - address: "1.2.3.5"
        enable: false
        location: "dc-us"
      - address: "10.0.0.1"
        enable: false
        location: "dc-eu"
      - address: "172.16.0.99"
        enable: false
        location: "dc-eu"
`
	f, err := os.CreateTemp("", "gslb_test_*.yml")
	assert.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.Write([]byte(tempYaml))
	assert.NoError(t, err)
	f.Close()

	g := &GSLB{
		Zones: map[string]string{"test": f.Name()},
	}
	mux := http.NewServeMux()
	g.RegisterAPIHandlers(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Enable by location
	resp, err := http.Post(ts.URL+"/api/backends/enable", "application/json", strings.NewReader(`{"location":"dc-eu"}`))
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	assert.NoError(t, err)
	assert.True(t, body["success"].(bool))
	backends, ok := body["backends"]
	assert.True(t, ok, "backends field should be present")
	beList, isList := backends.([]interface{})
	assert.True(t, isList, "backends should be a list (even if empty)")
	assert.Len(t, beList, 3)
	expectedBackends := []map[string]string{
		{"record": "test.example.com.", "address": "1.2.3.4"},
		{"record": "test.example.com.", "address": "10.0.0.1"},
		{"record": "test.example.com.", "address": "172.16.0.99"},
	}
	for _, expected := range expectedBackends {
		found := false
		for _, actual := range beList {
			be := actual.(map[string]interface{})
			if be["record"] == expected["record"] && be["address"] == expected["address"] {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected backend %+v not found in response", expected)
	}

	// Enable by IP prefix
	_ = os.WriteFile(f.Name(), []byte(tempYaml), 0644) // reset file
	resp2, err := http.Post(ts.URL+"/api/backends/enable", "application/json", strings.NewReader(`{"address_prefix":"1.2.3."}`))
	assert.NoError(t, err)
	defer resp2.Body.Close()
	var body3 map[string]interface{}
	err = json.NewDecoder(resp2.Body).Decode(&body3)
	assert.NoError(t, err)
	assert.True(t, body3["success"].(bool))
	backends2, ok2 := body3["backends"]
	assert.True(t, ok2, "backends field should be present")
	beList2, isList2 := backends2.([]interface{})
	assert.True(t, isList2, "backends should be a list (even if empty)")
	assert.Len(t, beList2, 2)
	expectedBackends2 := []map[string]string{
		{"record": "test.example.com.", "address": "1.2.3.4"},
		{"record": "test.example.com.", "address": "1.2.3.5"},
	}
	for _, expected := range expectedBackends2 {
		found := false
		for _, actual := range beList2 {
			be := actual.(map[string]interface{})
			if be["record"] == expected["record"] && be["address"] == expected["address"] {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected backend %+v not found in response", expected)
	}

	// Error case: missing body
	resp3, err := http.Post(ts.URL+"/api/backends/enable", "application/json", strings.NewReader(`{}`))
	assert.NoError(t, err)
	defer resp3.Body.Close()
	assert.Equal(t, 400, resp3.StatusCode)

	// Error case: wrong method
	req, _ := http.NewRequest("GET", ts.URL+"/api/backends/enable", nil)
	resp4, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp4.Body.Close()
	assert.Equal(t, 405, resp4.StatusCode)
}

func TestAPIBackendsAuthRequired(t *testing.T) {
	// Test that /api/backends/disable and /api/backends/enable require HTTP Basic Auth if configured
	tempYaml := `records:
  test.example.com.:
    backends:
      - address: "1.2.3.4"
        enable: true
        location: "dc-eu"
`
	f, err := os.CreateTemp("", "gslb_test_*.yml")
	assert.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.Write([]byte(tempYaml))
	assert.NoError(t, err)
	f.Close()

	g := &GSLB{
		Zones:        map[string]string{"test": f.Name()},
		APIBasicUser: "admin",
		APIBasicPass: "secret",
	}
	mux := http.NewServeMux()
	g.RegisterAPIHandlers(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// No auth
	resp, err := http.Post(ts.URL+"/api/backends/disable", "application/json", strings.NewReader(`{"location":"dc-eu"}`))
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 401, resp.StatusCode)

	resp2, err := http.Post(ts.URL+"/api/backends/enable", "application/json", strings.NewReader(`{"location":"dc-eu"}`))
	assert.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, 401, resp2.StatusCode)

	// Wrong auth
	req, _ := http.NewRequest("POST", ts.URL+"/api/backends/disable", strings.NewReader(`{"location":"dc-eu"}`))
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("bad:creds")))
	resp3, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp3.Body.Close()
	assert.Equal(t, 401, resp3.StatusCode)

	req2, _ := http.NewRequest("POST", ts.URL+"/api/backends/enable", strings.NewReader(`{"location":"dc-eu"}`))
	req2.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("bad:creds")))
	resp4, err := http.DefaultClient.Do(req2)
	assert.NoError(t, err)
	defer resp4.Body.Close()
	assert.Equal(t, 401, resp4.StatusCode)

	// Correct auth
	req3, _ := http.NewRequest("POST", ts.URL+"/api/backends/disable", strings.NewReader(`{"location":"dc-eu"}`))
	req3.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
	resp5, err := http.DefaultClient.Do(req3)
	assert.NoError(t, err)
	defer resp5.Body.Close()
	assert.Equal(t, 200, resp5.StatusCode)

	req4, _ := http.NewRequest("POST", ts.URL+"/api/backends/enable", strings.NewReader(`{"location":"dc-eu"}`))
	req4.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
	resp6, err := http.DefaultClient.Do(req4)
	assert.NoError(t, err)
	defer resp6.Body.Close()
	assert.Equal(t, 200, resp6.StatusCode)
}

func TestAPIDisableBackendsByTags(t *testing.T) {
	tempYaml := `records:
  test.example.com.:
    backends:
      - address: "1.2.3.4"
        enable: true
        tags: ["prod", "ssd"]
      - address: "1.2.3.5"
        enable: true
        tags: ["test", "hdd"]
      - address: "10.0.0.1"
        enable: true
        tags: ["prod", "hdd"]
      - address: "172.16.0.99"
        enable: true
        tags: ["dev"]
`
	f, err := os.CreateTemp("", "gslb_test_tags_*.yml")
	assert.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.Write([]byte(tempYaml))
	assert.NoError(t, err)
	f.Close()

	g := &GSLB{
		Zones: map[string]string{"test": f.Name()},
	}
	mux := http.NewServeMux()
	g.RegisterAPIHandlers(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Désactivation par tags
	resp, err := http.Post(ts.URL+"/api/backends/disable", "application/json", strings.NewReader(`{"tags":["prod","ssd"]}`))
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	assert.NoError(t, err)
	assert.True(t, body["success"].(bool))
	backends, ok := body["backends"]
	assert.True(t, ok, "backends field should be present")
	beList, isList := backends.([]interface{})
	assert.True(t, isList, "backends should be a list (even if empty)")
	assert.Len(t, beList, 2)
	expectedBackends := []map[string]string{
		{"record": "test.example.com.", "address": "1.2.3.4"},
		{"record": "test.example.com.", "address": "10.0.0.1"},
	}
	for _, expected := range expectedBackends {
		found := false
		for _, actual := range beList {
			be := actual.(map[string]interface{})
			if be["record"] == expected["record"] && be["address"] == expected["address"] {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected backend %+v not found in response", expected)
	}
}

// MockHealthCheckAPI always returns true and type "mock"
type MockHealthCheckAPI struct{}

func (m *MockHealthCheckAPI) PerformCheck(backend *Backend, fqdn string, maxRetries int) bool {
	return true
}
func (m *MockHealthCheckAPI) GetType() string                      { return "mock" }
func (m *MockHealthCheckAPI) Equals(other GenericHealthCheck) bool { return true }
