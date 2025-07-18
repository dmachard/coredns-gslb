package gslb

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLuaHealthCheck_Success(t *testing.T) {
	check := &LuaHealthCheck{
		Script:  `return true`,
		Timeout: 2 * time.Second,
	}
	backend := &Backend{Address: "127.0.0.1", Priority: 1, Enable: true}
	result := check.PerformCheck(backend, "test.local.", 1)
	if !result {
		t.Errorf("Expected Lua healthcheck to succeed")
	}
}

func TestLuaHealthCheck_Fail(t *testing.T) {
	check := &LuaHealthCheck{
		Script:  `return false`,
		Timeout: 2 * time.Second,
	}
	backend := &Backend{Address: "127.0.0.1", Priority: 1, Enable: true}
	result := check.PerformCheck(backend, "test.local.", 1)
	if result {
		t.Errorf("Expected Lua healthcheck to fail")
	}
}

func TestLuaHealthCheck_Timeout(t *testing.T) {
	check := &LuaHealthCheck{
		Script:  `local t=os.time(); while os.time()-t<2 do end; return true`,
		Timeout: 1 * time.Second,
	}
	backend := &Backend{Address: "127.0.0.1", Priority: 1, Enable: true}
	result := check.PerformCheck(backend, "test.local.", 1)
	if result {
		t.Errorf("Expected Lua healthcheck to timeout and fail")
	}
}

func TestLuaHealthCheck_BackendVars(t *testing.T) {
	check := &LuaHealthCheck{
		Script:  `if backend.address == "1.2.3.4" and backend.priority == 42 then return true else return false end`,
		Timeout: 2 * time.Second,
	}
	backend := &Backend{Address: "1.2.3.4", Priority: 42, Enable: true}
	result := check.PerformCheck(backend, "fqdn.test.", 1)
	if !result {
		t.Errorf("Expected Lua healthcheck to see backend variables")
	}
}

func TestLuaHealthCheck_HttpGet_JsonDecode(t *testing.T) {
	// Crée un serveur HTTP de test qui retourne du JSON
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"green","number_of_nodes":3}`)
	}))
	defer ts.Close()

	// Script Lua qui utilise http_get et json_decode
	script := fmt.Sprintf(`
		local health = json_decode(http_get("%s"))
		if health.status == "green" and health.number_of_nodes == 3 then
		  return true
		else
		  return false
		end
	`, ts.URL)

	check := &LuaHealthCheck{
		Script:  script,
		Timeout: 2 * time.Second,
	}
	backend := &Backend{Address: "127.0.0.1", Priority: 1, Enable: true}
	result := check.PerformCheck(backend, "fqdn.test.", 1)
	if !result {
		t.Errorf("Expected Lua healthcheck to succeed with http_get and json_decode")
	}
}

func TestLuaHealthCheck_MetricGet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "nginx_connections_active 42\n")
	}))
	defer ts.Close()

	script := fmt.Sprintf(`
		local value = metric_get("%s", "nginx_connections_active")
		if value and value == 42 then return true end
		return false
	`, ts.URL)

	check := &LuaHealthCheck{
		Script:  script,
		Timeout: 2 * time.Second,
	}
	backend := &Backend{Address: "127.0.0.1", Priority: 1, Enable: true}
	result := check.PerformCheck(backend, "fqdn.test.", 1)
	if !result {
		t.Errorf("Expected Lua healthcheck to succeed with metric_get")
	}
}

func TestLuaHealthCheck_HttpGet_Simple(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"green"}`))
	}))
	defer ts.Close()

	script := fmt.Sprintf(`
		local body = http_get("%s")
		local health = json_decode(body)
		if health and health.status == "green" then return true end
		return false
	`, ts.URL)

	check := &LuaHealthCheck{
		Script:  script,
		Timeout: 2 * time.Second,
	}
	backend := &Backend{Address: "127.0.0.1", Priority: 1, Enable: true}
	result := check.PerformCheck(backend, "fqdn.test.", 1)
	if !result {
		t.Errorf("Expected Lua healthcheck to succeed with http_get simple")
	}
}

func TestLuaHealthCheck_HttpGet_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte(`{"status":"green"}`))
	}))
	defer ts.Close()

	script := fmt.Sprintf(`
		local body = http_get("%s", 1)
		return body == ""
	`, ts.URL)

	check := &LuaHealthCheck{
		Script:  script,
		Timeout: 3 * time.Second,
	}
	backend := &Backend{Address: "127.0.0.1", Priority: 1, Enable: true}
	result := check.PerformCheck(backend, "fqdn.test.", 1)
	if !result {
		t.Errorf("Expected Lua healthcheck to timeout with http_get")
	}
}

func TestLuaHealthCheck_HttpGet_Auth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "Basic dXNlcjpwYXNz" { // user:pass
			w.Write([]byte(`{"status":"green"}`))
		} else {
			w.WriteHeader(401)
		}
	}))
	defer ts.Close()

	script := fmt.Sprintf(`
		local body = http_get("%s", 2, "user", "pass")
		local health = json_decode(body)
		if health and health.status == "green" then return true end
		return false
	`, ts.URL)

	check := &LuaHealthCheck{
		Script:  script,
		Timeout: 2 * time.Second,
	}
	backend := &Backend{Address: "127.0.0.1", Priority: 1, Enable: true}
	result := check.PerformCheck(backend, "fqdn.test.", 1)
	if !result {
		t.Errorf("Expected Lua healthcheck to succeed with http_get auth")
	}
}
