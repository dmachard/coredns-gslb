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
		ObserveHealthcheck(fqdn, typeStr, address, start, result)
	}()

	L := gopherlua.NewState()
	defer L.Close()

	// Inject helpers
	L.SetGlobal("http_get", L.NewFunction(luaHTTPGet))
	L.SetGlobal("json_decode", L.NewFunction(luaJSONDecode))
	L.SetGlobal("metric_get", L.NewFunction(luaMetricGet))
	L.SetGlobal("ssh_exec", L.NewFunction(luaSSHExec))

	// Inject backend table
	backendTable := L.NewTable()
	L.SetField(backendTable, "address", gopherlua.LString(backend.Address))
	L.SetField(backendTable, "priority", gopherlua.LNumber(backend.Priority))
	L.SetGlobal("backend", backendTable)

	err := L.DoString(l.Script)
	if err != nil {
		return false
	}
	ret := L.Get(-1)
	if lv, ok := ret.(gopherlua.LBool); ok {
		result = bool(lv)
		return result
	}
	return false
}

// Helper: json_decode(str) in Lua
func luaJSONDecode(l *gopherlua.LState) int {
	str := l.ToString(1)
	var data map[string]interface{}
	err := json.Unmarshal([]byte(str), &data)
	if err != nil {
		l.Push(gopherlua.LNil)
		return 1
	}
	tbl := l.NewTable()
	for k, v := range data {
		switch val := v.(type) {
		case string:
			l.SetField(tbl, k, gopherlua.LString(val))
		case float64:
			l.SetField(tbl, k, gopherlua.LNumber(val))
		case bool:
			l.SetField(tbl, k, gopherlua.LBool(val))
		default:
			l.SetField(tbl, k, gopherlua.LString(fmt.Sprintf("%v", val)))
		}
	}
	l.Push(tbl)
	return 1
}

// Helper: prometheus_metric(url, metric_name)
func luaMetricGet(l *gopherlua.LState) int {
	url := l.ToString(1)
	metric := l.ToString(2)
	var timeout = 10 // default 10s
	var tlsVerify = true
	var user, pass string
	argc := l.GetTop()
	if argc >= 3 {
		timeout = l.ToInt(3)
	}
	if argc >= 4 {
		tlsVerify = l.ToBool(4)
	}
	if argc >= 6 {
		user = l.ToString(5)
		pass = l.ToString(6)
	}
	var client *http.Client
	if !tlsVerify {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{Transport: tr, Timeout: time.Duration(timeout) * time.Second}
	} else {
		client = &http.Client{Timeout: time.Duration(timeout) * time.Second}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		l.Push(gopherlua.LNil)
		return 1
	}
	if user != "" || pass != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
		req.Header.Add("Authorization", "Basic "+auth)
	}
	resp, err := client.Do(req)
	if err != nil {
		l.Push(gopherlua.LNil)
		return 1
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		l.Push(gopherlua.LNil)
		return 1
	}
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, metric+" ") || strings.HasPrefix(line, metric+"{") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.ParseFloat(fields[len(fields)-1], 64); err == nil {
					l.Push(gopherlua.LNumber(v))
					return 1
				}
				l.Push(gopherlua.LString(fields[len(fields)-1]))
				return 1
			}
		}
	}
	l.Push(gopherlua.LNil)
	return 1
}

// Helper: http_get(url) in Lua
func luaHTTPGet(l *gopherlua.LState) int {
	url := l.ToString(1)
	var timeout = 10 // default timeout 10s
	var user, pass string
	var tlsVerify = true
	argc := l.GetTop()
	if argc >= 2 {
		timeout = l.ToInt(2)
	}
	if argc >= 4 {
		user = l.ToString(3)
		pass = l.ToString(4)
	}
	if argc >= 5 {
		tlsVerify = l.ToBool(5)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		l.Push(gopherlua.LString(""))
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
		l.Push(gopherlua.LString(""))
		return 1
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		l.Push(gopherlua.LString(""))
		return 1
	}
	l.Push(gopherlua.LString(string(body)))
	return 1
}

// Helper: ssh_exec(host, user, password, command, [timeout_sec])
func luaSSHExec(l *gopherlua.LState) int {
	host := l.ToString(1)
	user := l.ToString(2)
	password := l.ToString(3)
	cmd := l.ToString(4)
	var timeout = 5 * time.Second
	argc := l.GetTop()
	if argc >= 5 {
		timeout = time.Duration(l.ToInt(5)) * time.Second
	}
	client, err := goph.NewConn(&goph.Config{
		User:     user,
		Addr:     host,
		Port:     22,
		Auth:     goph.Password(password),
		Timeout:  timeout,
		Callback: sshInsecureIgnoreHostKey,
	})
	if err != nil {
		l.Push(gopherlua.LString(""))
		return 1
	}
	defer client.Close()
	out, err := client.Run(cmd)
	if err != nil {
		l.Push(gopherlua.LString(""))
		return 1
	}
	l.Push(gopherlua.LString(string(out)))
	return 1
}

func sshInsecureIgnoreHostKey(host string, remote net.Addr, key ssh.PublicKey) error {
	return nil // Accept all keys (for healthcheck only)
}
