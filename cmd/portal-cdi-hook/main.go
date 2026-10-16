// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// OCIContainerState represents the JSON state passed to OCI lifecycle hooks on stdin.
type OCIContainerState struct {
	OCIVersion  string            `json:"ociVersion"`
	ID          string            `json:"id"`
	Status      string            `json:"status"`
	Pid         int               `json:"pid"`
	Bundle      string            `json:"bundle"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// HookConfig contains tunable knobs for iptables rule generation.
type HookConfig struct {
	ProxyPort      string
	ProxyHTTPSPort string
	ProxyUID       string
	InterceptPorts string
	EgressAction   string
	AllowMetadata  bool
	DNSServers     []string
}

func defaultHookConfig() HookConfig {
	return HookConfig{
		ProxyPort:      "8080",
		ProxyHTTPSPort: "8443",
		ProxyUID:       "1337",
		InterceptPorts: "80,443",
		EgressAction:   "REJECT",
		AllowMetadata:  false,
		DNSServers:     nil,
	}
}

func parseResolvConf(r io.Reader) []string {
	var ips []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" {
			ip := fields[1]
			if net.ParseIP(ip) != nil {
				ips = append(ips, ip)
			}
		}
	}
	return ips
}

func discoverDNSServers(pid int, sandboxUID string) []string {
	servers := []string{"10.96.0.10"}
	found := make(map[string]bool)
	found["10.96.0.10"] = true

	// Check kubelet pod resolv.conf on host
	if sandboxUID != "" {
		podResolv := fmt.Sprintf("/var/lib/kubelet/pods/%s/resolv.conf", sandboxUID)
		f, err := os.Open(podResolv)
		if err == nil {
			for _, ip := range parseResolvConf(f) {
				if !found[ip] {
					found[ip] = true
					servers = append(servers, ip)
				}
			}
			if closeErr := f.Close(); closeErr != nil {
				log.Printf("error closing %s: %v", podResolv, closeErr)
			}
		} else if !os.IsNotExist(err) {
			log.Printf("error opening %s: %v", podResolv, err)
		}
	}

	// Also check any kubelet pod resolv.conf
	matches, err := filepath.Glob("/var/lib/kubelet/pods/*/resolv.conf")
	if err == nil {
		for _, m := range matches {
			f, err := os.Open(m)
			if err == nil {
				for _, ip := range parseResolvConf(f) {
					if !found[ip] {
						found[ip] = true
						servers = append(servers, ip)
					}
				}
				if closeErr := f.Close(); closeErr != nil {
					log.Printf("error closing %s: %v", m, closeErr)
				}
			} else if !os.IsNotExist(err) {
				log.Printf("error opening %s: %v", m, err)
			}
		}
	} else {
		log.Printf("error globbing pod resolv.conf: %v", err)
	}

	return servers
}

func generateRules(cfg HookConfig, family string) string {
	var b strings.Builder

	isV4 := family == "4" || family == "v4"
	var target, localhost, metadataIP, icmpProto string

	action := strings.ToUpper(cfg.EgressAction)
	if action == "DROP" {
		target = "DROP"
	} else if isV4 {
		target = "REJECT --reject-with icmp-port-unreachable"
	} else {
		target = "REJECT --reject-with icmp6-port-unreachable"
	}

	if isV4 {
		localhost = "127.0.0.1/32"
		metadataIP = "169.254.0.0/16"
		icmpProto = "icmp"
	} else {
		localhost = "::1/128"
		metadataIP = "fd00:ec2::254/128"
		icmpProto = "ipv6-icmp"
	}

	// NAT Table
	b.WriteString("*nat\n")
	b.WriteString(":PREROUTING ACCEPT [0:0]\n")
	b.WriteString(":INPUT ACCEPT [0:0]\n")
	b.WriteString(":OUTPUT ACCEPT [0:0]\n")
	b.WriteString(":POSTROUTING ACCEPT [0:0]\n")
	b.WriteString(":PORTAL_OUTPUT - [0:0]\n")
	b.WriteString("-F PORTAL_OUTPUT\n")
	b.WriteString("-A OUTPUT -p tcp -j PORTAL_OUTPUT\n")
	b.WriteString("-A PORTAL_OUTPUT -o lo -j RETURN\n")
	b.WriteString(fmt.Sprintf("-A PORTAL_OUTPUT -m owner --uid-owner %s -j RETURN\n", cfg.ProxyUID))

	if cfg.InterceptPorts == "*" {
		b.WriteString(fmt.Sprintf("-A PORTAL_OUTPUT -p tcp --dport 443 -j REDIRECT --to-ports %s\n", cfg.ProxyHTTPSPort))
		b.WriteString(fmt.Sprintf("-A PORTAL_OUTPUT -p tcp ! --dport 443 -j REDIRECT --to-ports %s\n", cfg.ProxyPort))
	} else {
		ports := strings.Split(cfg.InterceptPorts, ",")
		for _, p := range ports {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if p == "443" {
				b.WriteString(fmt.Sprintf("-A PORTAL_OUTPUT -p tcp --dport 443 -j REDIRECT --to-ports %s\n", cfg.ProxyHTTPSPort))
			} else {
				b.WriteString(fmt.Sprintf("-A PORTAL_OUTPUT -p tcp --dport %s -j REDIRECT --to-ports %s\n", p, cfg.ProxyPort))
			}
		}
	}
	b.WriteString("COMMIT\n")

	// Filter Table
	b.WriteString("*filter\n")
	b.WriteString(":INPUT ACCEPT [0:0]\n")
	b.WriteString(":FORWARD ACCEPT [0:0]\n")
	b.WriteString(":OUTPUT ACCEPT [0:0]\n")
	b.WriteString(":PORTAL_EGRESS - [0:0]\n")
	b.WriteString("-F PORTAL_EGRESS\n")
	b.WriteString("-A OUTPUT -j PORTAL_EGRESS\n")
	b.WriteString("-A PORTAL_EGRESS -o lo -j ACCEPT\n")
	b.WriteString(fmt.Sprintf("-A PORTAL_EGRESS -d %s -j ACCEPT\n", localhost))
	b.WriteString(fmt.Sprintf("-A PORTAL_EGRESS -d %s -p tcp --dport %s -j ACCEPT\n", localhost, cfg.ProxyPort))
	b.WriteString(fmt.Sprintf("-A PORTAL_EGRESS -d %s -p tcp --dport %s -j ACCEPT\n", localhost, cfg.ProxyHTTPSPort))
	b.WriteString("-A PORTAL_EGRESS -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT\n")
	b.WriteString(fmt.Sprintf("-A PORTAL_EGRESS -m owner --uid-owner %s -j ACCEPT\n", cfg.ProxyUID))

	if cfg.AllowMetadata {
		b.WriteString(fmt.Sprintf("-A PORTAL_EGRESS -d %s -j ACCEPT\n", metadataIP))
	} else {
		b.WriteString(fmt.Sprintf("-A PORTAL_EGRESS -d %s -j %s\n", metadataIP, target))
	}

	for _, dnsIP := range cfg.DNSServers {
		parsed := net.ParseIP(dnsIP)
		if parsed == nil {
			continue
		}
		isIP4 := parsed.To4() != nil
		if (isV4 && isIP4) || (!isV4 && !isIP4) {
			b.WriteString(fmt.Sprintf("-A PORTAL_EGRESS -d %s -p udp --dport 53 -j ACCEPT\n", dnsIP))
			b.WriteString(fmt.Sprintf("-A PORTAL_EGRESS -d %s -p tcp --dport 53 -j ACCEPT\n", dnsIP))
		}
	}

	b.WriteString(fmt.Sprintf("-A PORTAL_EGRESS -p %s -j %s\n", icmpProto, target))
	b.WriteString(fmt.Sprintf("-A PORTAL_EGRESS -p tcp -j %s\n", target))
	b.WriteString(fmt.Sprintf("-A PORTAL_EGRESS -p udp -j %s\n", target))
	b.WriteString("COMMIT\n")

	return b.String()
}

func findBinary(name string, fallbacks []string) (string, error) {
	if os.Getenv("PATH") == "" {
		if err := os.Setenv("PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"); err != nil {
			log.Printf("error setting PATH: %v", err)
		}
	}
	p, err := exec.LookPath(name)
	if err == nil {
		return p, nil
	}
	for _, fb := range fallbacks {
		if _, statErr := os.Stat(fb); statErr == nil {
			return fb, nil
		} else if !os.IsNotExist(statErr) {
			log.Printf("error stating fallback %s: %v", fb, statErr)
		}
	}
	return "", fmt.Errorf("binary %s not found: %w", name, err)
}

func applyRulesInNetns(pid int, rules string, isV6 bool) error {
	bin := "iptables-restore"
	if isV6 {
		bin = "ip6tables-restore"
	}

	nsenterPath, err := findBinary("nsenter", []string{"/usr/bin/nsenter", "/bin/nsenter"})
	if err != nil {
		return fmt.Errorf("nsenter not found: %w", err)
	}

	iptablesPath, err := findBinary(bin, []string{"/usr/sbin/" + bin, "/sbin/" + bin, "/usr/bin/" + bin})
	if err != nil {
		return fmt.Errorf("%s not found: %w", bin, err)
	}

	cmd := exec.Command(nsenterPath, "-t", fmt.Sprintf("%d", pid), "-n", "--", iptablesPath)
	cmd.Stdin = strings.NewReader(rules)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed executing %s via nsenter: %w (stderr: %s)", bin, err, stderr.String())
	}
	return nil
}

func main() {
	generateFlag := flag.String("generate", "", "Generate rules for family (4 or 6) and print to stdout")
	flag.Parse()

	cfg := defaultHookConfig()

	if *generateFlag != "" {
		cfg.DNSServers = []string{"10.96.0.10"}
		fmt.Print(generateRules(cfg, *generateFlag))
		return
	}

	// Read OCI state from stdin
	stateBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("failed reading OCI state from stdin: %v", err)
	}

	// Log hook execution if log file is writable
	logFile, err := os.OpenFile("/var/log/portal-cdi-hook.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		defer func() {
			if closeErr := logFile.Close(); closeErr != nil {
				log.Printf("error closing log file: %v", closeErr)
			}
		}()
		if _, writeErr := fmt.Fprintf(logFile, "[%s] Received hook invocation: %s\n", filepath.Base(os.Args[0]), string(stateBytes)); writeErr != nil {
			log.Printf("error writing log: %v", writeErr)
		}
	}

	var state OCIContainerState
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		log.Fatalf("failed unmarshaling OCI state: %v", err)
	}

	if state.Pid <= 0 {
		// No valid PID, nothing to configure in netns
		return
	}

	// Discover DNS servers for this container
	sandboxUID := ""
	if state.Annotations != nil {
		sandboxUID = state.Annotations["io.kubernetes.cri.sandbox-uid"]
	}
	cfg.DNSServers = discoverDNSServers(state.Pid, sandboxUID)

	// Generate and apply IPv4 rules
	v4Rules := generateRules(cfg, "4")
	if logFile != nil {
		if _, writeErr := fmt.Fprintf(logFile, "[%s] PID: %d, sandboxUID: %s, DNS: %v\n", filepath.Base(os.Args[0]), state.Pid, sandboxUID, cfg.DNSServers); writeErr != nil {
			log.Printf("error writing log: %v", writeErr)
		}
		if _, writeErr := fmt.Fprintf(logFile, "[%s] v4Rules:\n%s\n", filepath.Base(os.Args[0]), v4Rules); writeErr != nil {
			log.Printf("error writing log: %v", writeErr)
		}
	}

	if err := applyRulesInNetns(state.Pid, v4Rules, false); err != nil {
		log.Printf("Warning: failed to apply IPv4 rules in netns of PID %d: %v", state.Pid, err)
	}

	// Generate and apply IPv6 rules
	v6Rules := generateRules(cfg, "6")
	if err := applyRulesInNetns(state.Pid, v6Rules, true); err != nil {
		// IPv6 may not be enabled on kernel, log as warning
		log.Printf("Note: IPv6 rules skipped or failed for PID %d: %v", state.Pid, err)
	}
}
