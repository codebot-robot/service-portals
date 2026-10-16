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
	"strings"
	"testing"
)

func TestParseResolvConf(t *testing.T) {
	resolvContent := `
# Kubernetes DNS
search default.svc.cluster.local svc.cluster.local
nameserver 10.96.0.10
nameserver 2001:db8::53
options ndots:5
`
	ips := parseResolvConf(strings.NewReader(resolvContent))
	if len(ips) != 2 {
		t.Fatalf("expected 2 nameservers, got %d: %v", len(ips), ips)
	}
	if ips[0] != "10.96.0.10" {
		t.Errorf("expected 10.96.0.10, got %s", ips[0])
	}
	if ips[1] != "2001:db8::53" {
		t.Errorf("expected 2001:db8::53, got %s", ips[1])
	}
}

func TestGenerateRules_IPv4(t *testing.T) {
	cfg := defaultHookConfig()
	cfg.DNSServers = []string{"10.96.0.10"}

	rules := generateRules(cfg, "4")

	expected := []string{
		"*nat",
		":PORTAL_OUTPUT - [0:0]",
		"-F PORTAL_OUTPUT",
		"-A OUTPUT -p tcp -j PORTAL_OUTPUT",
		"-A PORTAL_OUTPUT -o lo -j RETURN",
		"-A PORTAL_OUTPUT -m owner --uid-owner 1337 -j RETURN",
		"-A PORTAL_OUTPUT -p tcp --dport 80 -j REDIRECT --to-ports 8080",
		"-A PORTAL_OUTPUT -p tcp --dport 443 -j REDIRECT --to-ports 8443",
		"COMMIT",
		"*filter",
		":PORTAL_EGRESS - [0:0]",
		"-F PORTAL_EGRESS",
		"-A OUTPUT -j PORTAL_EGRESS",
		"-A PORTAL_EGRESS -o lo -j ACCEPT",
		"-A PORTAL_EGRESS -d 127.0.0.1/32 -j ACCEPT",
		"-A PORTAL_EGRESS -d 127.0.0.1/32 -p tcp --dport 8080 -j ACCEPT",
		"-A PORTAL_EGRESS -d 127.0.0.1/32 -p tcp --dport 8443 -j ACCEPT",
		"-A PORTAL_EGRESS -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT",
		"-A PORTAL_EGRESS -m owner --uid-owner 1337 -j ACCEPT",
		"-A PORTAL_EGRESS -d 169.254.0.0/16 -j REJECT --reject-with icmp-port-unreachable",
		"-A PORTAL_EGRESS -d 10.96.0.10 -p udp --dport 53 -j ACCEPT",
		"-A PORTAL_EGRESS -d 10.96.0.10 -p tcp --dport 53 -j ACCEPT",
		"-A PORTAL_EGRESS -p icmp -j REJECT --reject-with icmp-port-unreachable",
		"-A PORTAL_EGRESS -p tcp -j REJECT --reject-with icmp-port-unreachable",
		"-A PORTAL_EGRESS -p udp -j REJECT --reject-with icmp-port-unreachable",
	}

	for _, s := range expected {
		if !strings.Contains(rules, s) {
			t.Errorf("expected IPv4 rules to contain %q, but did not.\nRules:\n%s", s, rules)
		}
	}
}

func TestGenerateRules_IPv6(t *testing.T) {
	cfg := defaultHookConfig()
	cfg.DNSServers = []string{"2001:db8::53"}

	rules := generateRules(cfg, "6")

	expected := []string{
		"*nat",
		":PORTAL_OUTPUT - [0:0]",
		"-F PORTAL_OUTPUT",
		"-A OUTPUT -p tcp -j PORTAL_OUTPUT",
		"-A PORTAL_OUTPUT -o lo -j RETURN",
		"-A PORTAL_OUTPUT -m owner --uid-owner 1337 -j RETURN",
		"-A PORTAL_OUTPUT -p tcp --dport 80 -j REDIRECT --to-ports 8080",
		"-A PORTAL_OUTPUT -p tcp --dport 443 -j REDIRECT --to-ports 8443",
		"COMMIT",
		"*filter",
		":PORTAL_EGRESS - [0:0]",
		"-F PORTAL_EGRESS",
		"-A OUTPUT -j PORTAL_EGRESS",
		"-A PORTAL_EGRESS -o lo -j ACCEPT",
		"-A PORTAL_EGRESS -d ::1/128 -j ACCEPT",
		"-A PORTAL_EGRESS -d ::1/128 -p tcp --dport 8080 -j ACCEPT",
		"-A PORTAL_EGRESS -d ::1/128 -p tcp --dport 8443 -j ACCEPT",
		"-A PORTAL_EGRESS -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT",
		"-A PORTAL_EGRESS -m owner --uid-owner 1337 -j ACCEPT",
		"-A PORTAL_EGRESS -d fd00:ec2::254/128 -j REJECT --reject-with icmp6-port-unreachable",
		"-A PORTAL_EGRESS -d 2001:db8::53 -p udp --dport 53 -j ACCEPT",
		"-A PORTAL_EGRESS -d 2001:db8::53 -p tcp --dport 53 -j ACCEPT",
		"-A PORTAL_EGRESS -p ipv6-icmp -j REJECT --reject-with icmp6-port-unreachable",
		"-A PORTAL_EGRESS -p tcp -j REJECT --reject-with icmp6-port-unreachable",
		"-A PORTAL_EGRESS -p udp -j REJECT --reject-with icmp6-port-unreachable",
	}

	for _, s := range expected {
		if !strings.Contains(rules, s) {
			t.Errorf("expected IPv6 rules to contain %q, but did not.\nRules:\n%s", s, rules)
		}
	}
}

func TestGenerateRules_DropAction(t *testing.T) {
	cfg := defaultHookConfig()
	cfg.EgressAction = "DROP"
	cfg.DNSServers = []string{"10.96.0.10"}

	rules := generateRules(cfg, "4")

	if strings.Contains(rules, "REJECT") {
		t.Errorf("expected no REJECT when action is DROP, got:\n%s", rules)
	}
	if !strings.Contains(rules, "-A PORTAL_EGRESS -p tcp -j DROP") {
		t.Errorf("expected DROP rule for TCP")
	}
}
