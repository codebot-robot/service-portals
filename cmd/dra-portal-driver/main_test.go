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
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	drav1beta1 "k8s.io/kubelet/pkg/apis/dra/v1beta1"
	regv1 "k8s.io/kubelet/pkg/apis/pluginregistration/v1"
)

func TestGenerateCACertificate(t *testing.T) {
	tempDir := t.TempDir()
	certDir := filepath.Join(tempDir, "ca")
	keyDir := filepath.Join(tempDir, "ca-private")

	err := GenerateCACertificate(certDir, keyDir, -1, -1)
	if err != nil {
		t.Fatalf("GenerateCACertificate failed: %v", err)
	}

	certPath := filepath.Join(certDir, "tls.crt")
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("failed reading tls.crt: %v", err)
	}
	block, _ := pem.Decode(certBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("invalid certificate PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed parsing certificate: %v", err)
	}
	if !cert.IsCA {
		t.Errorf("expected cert.IsCA to be true")
	}
	if cert.Subject.CommonName != "Service Portal Root CA" {
		t.Errorf("expected CommonName 'Service Portal Root CA', got %s", cert.Subject.CommonName)
	}

	keyPath := filepath.Join(keyDir, "tls.key")
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("failed reading tls.key: %v", err)
	}
	keyBlock, _ := pem.Decode(keyBytes)
	if keyBlock == nil || keyBlock.Type != "PRIVATE KEY" {
		t.Fatalf("invalid private key PEM block")
	}
}

func TestGenerateCDISpec(t *testing.T) {
	spec := GenerateCDISpec("/var/run/service-portal/ca", "/usr/local/bin/portal-cdi-hook")
	if spec.CDIVersion != "0.6.0" {
		t.Errorf("expected CDI version 0.6.0, got %s", spec.CDIVersion)
	}
	if spec.Kind != DefaultCDIKind {
		t.Errorf("expected kind %s, got %s", DefaultCDIKind, spec.Kind)
	}
	if len(spec.Devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(spec.Devices))
	}

	dev := spec.Devices[0]
	if dev.Name != DefaultCDIDevice {
		t.Errorf("expected device name %s, got %s", DefaultCDIDevice, dev.Name)
	}
	if len(dev.ContainerEdits.Env) == 0 || dev.ContainerEdits.Env[0] != "SSL_CERT_FILE=/etc/service-portal/ca/tls.crt" {
		t.Errorf("expected SSL_CERT_FILE in env, got: %v", dev.ContainerEdits.Env)
	}
	if len(dev.ContainerEdits.Mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(dev.ContainerEdits.Mounts))
	}
	if dev.ContainerEdits.Mounts[0].ContainerPath != "/etc/service-portal/ca" {
		t.Errorf("expected containerPath /etc/service-portal/ca, got %s", dev.ContainerEdits.Mounts[0].ContainerPath)
	}
	if len(dev.ContainerEdits.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(dev.ContainerEdits.Hooks))
	}
	if dev.ContainerEdits.Hooks[0].HookName != "createRuntime" {
		t.Errorf("expected hookName createRuntime, got %s", dev.ContainerEdits.Hooks[0].HookName)
	}
	if dev.ContainerEdits.Hooks[0].Path != "/usr/local/bin/portal-cdi-hook" {
		t.Errorf("expected hook path /usr/local/bin/portal-cdi-hook, got %s", dev.ContainerEdits.Hooks[0].Path)
	}
}

func TestWriteCDISpec(t *testing.T) {
	tempDir := t.TempDir()
	cdiDir := filepath.Join(tempDir, "cdi")

	err := WriteCDISpec(cdiDir, "/var/run/service-portal/ca", "/usr/local/bin/portal-cdi-hook")
	if err != nil {
		t.Fatalf("WriteCDISpec failed: %v", err)
	}

	cdiFile := filepath.Join(cdiDir, "portal-egress.json")
	data, err := os.ReadFile(cdiFile)
	if err != nil {
		t.Fatalf("failed reading written CDI spec: %v", err)
	}

	var parsed CDISpec
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed unmarshaling written CDI spec: %v", err)
	}
	if parsed.Kind != DefaultCDIKind {
		t.Errorf("expected kind %s, got %s", DefaultCDIKind, parsed.Kind)
	}
}

func TestDRAServer_PrepareAndUnprepare(t *testing.T) {
	srv := &DRAServer{DriverName: DefaultDriverName}

	claimUID := "test-claim-uid-1234"
	req := &drav1beta1.NodePrepareResourcesRequest{
		Claims: []*drav1beta1.Claim{
			{
				Namespace: "default",
				Name:      "test-claim",
				Uid:       claimUID,
			},
		},
	}

	resp, err := srv.NodePrepareResources(t.Context(), req)
	if err != nil {
		t.Fatalf("NodePrepareResources failed: %v", err)
	}
	if len(resp.Claims) != 1 {
		t.Fatalf("expected 1 claim response, got %d", len(resp.Claims))
	}
	claimResp, ok := resp.Claims[claimUID]
	if !ok {
		t.Fatalf("missing response for claim %s", claimUID)
	}
	if len(claimResp.Devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(claimResp.Devices))
	}
	expectedCDIDevice := DefaultCDIKind + "=" + DefaultCDIDevice
	if len(claimResp.Devices[0].CdiDeviceIds) != 1 || claimResp.Devices[0].CdiDeviceIds[0] != expectedCDIDevice {
		t.Errorf("expected CDI device %s, got %v", expectedCDIDevice, claimResp.Devices[0].CdiDeviceIds)
	}

	unprepReq := &drav1beta1.NodeUnprepareResourcesRequest{
		Claims: []*drav1beta1.Claim{
			{
				Namespace: "default",
				Name:      "test-claim",
				Uid:       claimUID,
			},
		},
	}
	unprepResp, err := srv.NodeUnprepareResources(t.Context(), unprepReq)
	if err != nil {
		t.Fatalf("NodeUnprepareResources failed: %v", err)
	}
	if len(unprepResp.Claims) != 1 {
		t.Fatalf("expected 1 unprep claim response, got %d", len(unprepResp.Claims))
	}
}

func TestRegServer_GetInfo(t *testing.T) {
	srv := &RegServer{
		DriverName:   DefaultDriverName,
		DriverSocket: "/var/lib/kubelet/plugins/" + DefaultDriverName + "/dra.sock",
	}

	info, err := srv.GetInfo(t.Context(), &regv1.InfoRequest{})
	if err != nil {
		t.Fatalf("GetInfo failed: %v", err)
	}
	if info.Type != regv1.DRAPlugin {
		t.Errorf("expected plugin type DRAPlugin, got %s", info.Type)
	}
	if info.Name != DefaultDriverName {
		t.Errorf("expected driver name %s, got %s", DefaultDriverName, info.Name)
	}
	if info.Endpoint != srv.DriverSocket {
		t.Errorf("expected endpoint %s, got %s", srv.DriverSocket, info.Endpoint)
	}
	if len(info.SupportedVersions) != 2 || info.SupportedVersions[0] != "v1alpha4" || info.SupportedVersions[1] != "v1beta1" {
		t.Errorf("unexpected supported versions: %v", info.SupportedVersions)
	}

	statusResp, err := srv.NotifyRegistrationStatus(t.Context(), &regv1.RegistrationStatus{
		PluginRegistered: true,
	})
	if err != nil {
		t.Fatalf("NotifyRegistrationStatus failed: %v", err)
	}
	if statusResp == nil {
		t.Errorf("expected non-nil response from NotifyRegistrationStatus")
	}
}
