package gslb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"crypto/tls"

	"github.com/melbahja/goph"
	gopherlua "github.com/yuin/gopher-lua"
	ssh "golang.org/x/crypto/ssh"
)

type LuaHealthCheck struct {
	Script  string        `yaml:"script"`
	Timeout time.Duration `yaml:"timeout"`
}

func (l *LuaHealthCheck) SetDefault() {
	if l.Timeout == 0 {
		l.Timeout = 5 * time.Second
	}
}

func (l *LuaHealthCheck) GetType() string {
	return "lua"
}

func (l *LuaHealthCheck) Equals(other GenericHealthCheck) bool {
	otherL, ok := other.(*LuaHealthCheck)
	if !ok {
		return false
	}
	return l.Script == otherL.Script && l.Timeout == otherL.Timeout
}

func (l *LuaHealthCheck) PerformCheck(backend *Backend, fqdn string, maxRetries int) bool {
	l.SetDefault()
	typeStr := l.GetType()
	address := backend.Address
	start := time.Now()
	result := false
	defer func() {
		ObserveHealthcheck(typeStr, address, start, result)
	}()

	for i := 0; i < maxRetries; i++ {
		resultChan := make(chan struct {
			result bool
			err    error
		}, 1)
		go func() {
			L := gopherlua.NewState()
			defer L.Close()

			// Inject helpers
			L.SetGlobal("http_get", L.NewFunction(luaHTTPGet))
			L.SetGlobal("http_get_ex", L.NewFunction(luaHTTPGetEx))
			L.SetGlobal("http_get_auth", L.NewFunction(luaHTTPGetAuth))
			L.SetGlobal("json_decode", L.NewFunction(luaJSONDecode))
			L.SetGlobal("metric_get", L.NewFunction(luaMetricGet))
			L.SetGlobal("ssh_exec", L.NewFunction(luaSSHExec))
			L.SetGlobal("tls_cert_days_left", L.NewFunction(luaTLSCertDaysLeft))

			// Inject backend table
			backendTable := L.NewTable()
			L.SetField(backendTable, "address", gopherlua.LString(backend.Address))
			L.SetField(backendTable, "priority", gopherlua.LNumber(backend.Priority))
			L.SetGlobal("backend", backendTable)

			err := L.DoString(l.Script)
			if err != nil {
				resultChan <- struct {
					result bool
					err    error
				}{false, err}
				return
			}
			ret := L.Get(-1)
			if lv, ok := ret.(gopherlua.LBool); ok {
				resultChan <- struct {
					result bool
					err    error
				}{bool(lv), nil}
				return
			}
			resultChan <- struct {
				result bool
				err    error
			}{false, nil}
		}()
		select {
		case res := <-resultChan:
			if res.err != nil {
				return false
			}
			result = res.result
			return result
		case <-time.After(l.Timeout):
			return false
		}
	}
	return false
}

// Helper: http_get(url) in Lua
func luaHTTPGet(L *gopherlua.LState) int {
	url := L.ToString(1)
	var timeout int = 10 // default timeout 10s
	var user, pass string
	var tlsVerify = true
	argc := L.GetTop()
	if argc >= 2 {
		timeout = L.ToInt(2)
	}
	if argc >= 4 {
		user = L.ToString(3)
		pass = L.ToString(4)
	}
	if argc >= 5 {
		tlsVerify = L.ToBool(5)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		L.Push(gopherlua.LString(""))
		return 1
	}
	if user != "" || pass != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
		req.Header.Add("Authorization", "Basic "+auth)
	}
	client := http.DefaultClient
	if !tlsVerify {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{Transport: tr, Timeout: time.Duration(timeout) * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		L.Push(gopherlua.LString(""))
		return 1
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		L.Push(gopherlua.LString(""))
		return 1
	}
	L.Push(gopherlua.LString(string(body)))
	return 1
}

// Helper: http_get_ex(url, timeout_sec)
func luaHTTPGetEx(L *gopherlua.LState) int {
	url := L.ToString(1)
	timeout := L.ToInt(2)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		L.Push(gopherlua.LString(""))
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		L.Push(gopherlua.LString(""))
		return 1
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		L.Push(gopherlua.LString(""))
		return 1
	}
	L.Push(gopherlua.LString(string(body)))
	return 1
}

// Helper: http_get_auth(url, user, password, timeout_sec)
func luaHTTPGetAuth(L *gopherlua.LState) int {
	url := L.ToString(1)
	user := L.ToString(2)
	pass := L.ToString(3)
	timeout := L.ToInt(4)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		L.Push(gopherlua.LString(""))
		return 1
	}
	auth := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	req.Header.Add("Authorization", "Basic "+auth)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		L.Push(gopherlua.LString(""))
		return 1
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		L.Push(gopherlua.LString(""))
		return 1
	}
	L.Push(gopherlua.LString(string(body)))
	return 1
}

// Helper: json_decode(str) in Lua
func luaJSONDecode(L *gopherlua.LState) int {
	str := L.ToString(1)
	var data map[string]interface{}
	err := json.Unmarshal([]byte(str), &data)
	if err != nil {
		L.Push(gopherlua.LNil)
		return 1
	}
	tbl := L.NewTable()
	for k, v := range data {
		switch val := v.(type) {
		case string:
			L.SetField(tbl, k, gopherlua.LString(val))
		case float64:
			L.SetField(tbl, k, gopherlua.LNumber(val))
		case bool:
			L.SetField(tbl, k, gopherlua.LBool(val))
		default:
			L.SetField(tbl, k, gopherlua.LString(fmt.Sprintf("%v", val)))
		}
	}
	L.Push(tbl)
	return 1
}

// Helper: prometheus_metric(url, metric_name)
func luaMetricGet(L *gopherlua.LState) int {
	url := L.ToString(1)
	metric := L.ToString(2)
	resp, err := http.Get(url)
	if err != nil {
		L.Push(gopherlua.LNil)
		return 1
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		L.Push(gopherlua.LNil)
		return 1
	}
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, metric+" ") || strings.HasPrefix(line, metric+"{") {
			// Ex: metric 42.0
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.ParseFloat(fields[len(fields)-1], 64); err == nil {
					L.Push(gopherlua.LNumber(v))
					return 1
				}
				L.Push(gopherlua.LString(fields[len(fields)-1]))
				return 1
			}
		}
	}
	L.Push(gopherlua.LNil)
	return 1
}

// Helper: ssh_exec(host, user, password, command)
func luaSSHExec(L *gopherlua.LState) int {
	host := L.ToString(1)
	user := L.ToString(2)
	password := L.ToString(3)
	cmd := L.ToString(4)
	client, err := goph.NewConn(&goph.Config{
		User:     user,
		Addr:     host,
		Port:     22,
		Auth:     goph.Password(password),
		Timeout:  5 * time.Second,
		Callback: sshInsecureIgnoreHostKey,
	})
	if err != nil {
		L.Push(gopherlua.LString(""))
		return 1
	}
	defer client.Close()
	out, err := client.Run(cmd)
	if err != nil {
		L.Push(gopherlua.LString(""))
		return 1
	}
	L.Push(gopherlua.LString(string(out)))
	return 1
}

func sshInsecureIgnoreHostKey(host string, remote net.Addr, key ssh.PublicKey) error {
	return nil // Accept all keys (for healthcheck only)
}

// Helper: tls_cert_days_left(host, port)
func luaTLSCertDaysLeft(L *gopherlua.LState) int {
	host := L.ToString(1)
	port := L.ToInt(2)
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		L.Push(gopherlua.LNil)
		return 1
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		L.Push(gopherlua.LNil)
		return 1
	}
	notAfter := certs[0].NotAfter
	days := int(time.Until(notAfter).Hours() / 24)
	L.Push(gopherlua.LNumber(days))
	return 1
}
