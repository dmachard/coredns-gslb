package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

// Default Corefile path (can be overridden by CORE_DNS_COREFILE env var)
const defaultCorefilePath = "/coredns/Corefile"

// Config holds parsed Corefile values
type Config struct {
	User string
	Pass string
	TLS  bool
	Addr string
	Port string
}

// Parse Corefile for auth and API info
func parseCorefile() Config {
	path := os.Getenv("CORE_DNS_COREFILE")
	if path == "" {
		path = defaultCorefilePath
	}
	cfg := Config{Addr: "127.0.0.1", Port: "8080"}
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gslbctl] Warning: cannot open Corefile: %v\n", err)
		return cfg
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "api_basic_user") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				cfg.User = fields[1]
			}
		}
		if strings.HasPrefix(line, "api_basic_pass") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				cfg.Pass = fields[1]
			}
		}
		if strings.HasPrefix(line, "api_tls_cert") {
			fields := strings.Fields(line)
			if len(fields) > 1 && fields[1] != "" {
				cfg.TLS = true
			}
		}
		if strings.HasPrefix(line, "api_listen_addr") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				cfg.Addr = fields[1]
			}
		}
		if strings.HasPrefix(line, "api_listen_port") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				cfg.Port = fields[1]
			}
		}
	}
	return cfg
}

// Build API URL based on config
func apiURL(cfg Config) string {
	proto := "http"
	if cfg.TLS {
		proto = "https"
	}
	return fmt.Sprintf("%s://%s:%s", proto, cfg.Addr, cfg.Port)
}

// Add basic auth header if needed
func addAuth(req *http.Request, cfg Config) {
	if cfg.User != "" && cfg.Pass != "" {
		auth := cfg.User + ":" + cfg.Pass
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
	}
}

func main() {
	cfg := parseCorefile()
	api := apiURL(cfg)

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "backends":
		backendsCmd(os.Args[2:], api, cfg)
	case "status":
		statusCmd(api, cfg)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`Usage: gslbctl <command> [options]

Commands:
  backends enable   [--tags tag1,tag2] [--address addr] [--location loc]
  backends disable  [--tags tag1,tag2] [--address addr] [--location loc]
  status
`)
}

func backendsCmd(args []string, api string, cfg Config) {
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}
	sub := args[0]
	fs := flag.NewFlagSet("backends "+sub, flag.ExitOnError)
	tags := fs.String("tags", "", "Comma-separated list of tags")
	address := fs.String("address", "", "Backend address")
	location := fs.String("location", "", "Location string")
	fs.Parse(args[1:])

	var body = make(map[string]interface{})
	if *tags != "" {
		body["tags"] = strings.Split(*tags, ",")
	}
	if *address != "" {
		body["address_prefix"] = *address
	}
	if *location != "" {
		body["location"] = *location
	}
	jsonBody, _ := json.Marshal(body)

	var endpoint string
	switch sub {
	case "enable":
		endpoint = "/api/backends/enable"
	case "disable":
		endpoint = "/api/backends/disable"
	default:
		usage()
		os.Exit(1)
	}

	req, err := http.NewRequest("POST", api+endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		fmt.Fprintf(os.Stderr, "API error: %v\n", err)
		os.Exit(2)
	}
	addAuth(req, cfg)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "API error: %v\n", err)
		os.Exit(2)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	// Try to parse and print as table if possible

	type backendResp struct {
		Address string `json:"address"`
		Record  string `json:"record"`
	}
	type apiResp struct {
		Success  bool          `json:"success"`
		Backends []backendResp `json:"backends"`
		Error    string        `json:"error"`
	}
	var r apiResp
	if os.Getenv("GSLBCTL_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[gslbctl debug] Raw API response: %s\n", string(data))
	}
	if err := json.Unmarshal(data, &r); err == nil {
		if os.Getenv("GSLBCTL_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[gslbctl debug] Parsed struct: %+v\n", r)
		}
		if r.Success {
			if len(r.Backends) > 0 {
				w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
				fmt.Fprintln(w, "RECORD\tBACKEND")
				for _, be := range r.Backends {
					fmt.Fprintf(w, "%s\t%s\n", be.Record, be.Address)
				}
				w.Flush()
			} else {
				fmt.Println("No backends matched your criteria.")
			}
		} else {
			if r.Error != "" {
				fmt.Fprintf(os.Stderr, "Error: %s\n", r.Error)
			} else {
				fmt.Fprintf(os.Stderr, "Operation failed.\n")
			}
		}
		return
	}
	// fallback: raw output
	fmt.Println(string(data))
}

func statusCmd(api string, cfg Config) {
	req, err := http.NewRequest("GET", api+"/api/overview", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "API error: %v\n", err)
		os.Exit(2)
	}
	addAuth(req, cfg)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "API error: %v\n", err)
		os.Exit(2)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	// Try to parse and print as table

	type backend struct {
		Address         string `json:"address"`
		Alive           string `json:"alive"`
		LastHealthcheck string `json:"last_healthcheck"`
	}
	type record struct {
		Record   string    `json:"record"`
		Status   string    `json:"status"`
		Backends []backend `json:"backends"`
	}
	var overview map[string][]record
	if err := json.Unmarshal(data, &overview); err == nil {
		w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ZONE\tRECORD\tSTATUS\tBACKEND\tALIVE\tLAST_HEALTHCHECK")
		// Sort zones for stable output
		zones := make([]string, 0, len(overview))
		for zone := range overview {
			zones = append(zones, zone)
		}
		sort.Strings(zones)
		for _, zone := range zones {
			recs := overview[zone]
			for _, rec := range recs {
				for _, be := range rec.Backends {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", zone, rec.Record, rec.Status, be.Address, be.Alive, be.LastHealthcheck)
				}
			}
		}
		w.Flush()
		return
	}
	// fallback: pretty JSON
	var pretty bytes.Buffer
	err = json.Indent(&pretty, data, "", "  ")
	if err != nil {
		fmt.Println(string(data))
	} else {
		fmt.Println(pretty.String())
	}
}
