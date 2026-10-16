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
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"google.golang.org/grpc"
	drav1beta1 "k8s.io/kubelet/pkg/apis/dra/v1beta1"
	regv1 "k8s.io/kubelet/pkg/apis/pluginregistration/v1"
)

const (
	DefaultDriverName = "portal-egress.portals.gke.io"
	DefaultCDIKind    = "portals.gke.io/portal-egress"
	DefaultCDIDevice  = "portal-egress"
)

// CDISpec represents a Container Device Interface JSON spec.
type CDISpec struct {
	CDIVersion string      `json:"cdiVersion"`
	Kind       string      `json:"kind"`
	Devices    []CDIDevice `json:"devices"`
}

type CDIDevice struct {
	Name           string         `json:"name"`
	ContainerEdits ContainerEdits `json:"containerEdits"`
}

type ContainerEdits struct {
	Env    []string   `json:"env,omitempty"`
	Mounts []CDIMount `json:"mounts,omitempty"`
	Hooks  []CDIHook  `json:"hooks,omitempty"`
}

type CDIMount struct {
	HostPath      string   `json:"hostPath"`
	ContainerPath string   `json:"containerPath"`
	Options       []string `json:"options,omitempty"`
}

type CDIHook struct {
	HookName string   `json:"hookName"`
	Path     string   `json:"path"`
	Args     []string `json:"args,omitempty"`
}

// GenerateCACertificate generates a self-signed root CA certificate and private key.
func GenerateCACertificate(certDir, keyDir string, chownUID, chownGID int) error {
	certPath := filepath.Join(certDir, "tls.crt")
	keyPath := filepath.Join(keyDir, "tls.key")

	certExists := false
	if _, err := os.Stat(certPath); err == nil {
		certExists = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking cert path: %w", err)
	}

	keyExists := false
	if _, err := os.Stat(keyPath); err == nil {
		keyExists = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking key path: %w", err)
	}

	if certExists && keyExists {
		return nil
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Service Portal Dynamic CA"},
			CommonName:   "Service Portal Root CA",
		},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(time.Hour * 24 * 365 * 10), // 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	keyBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("failed to create cert directory: %w", err)
	}
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return fmt.Errorf("failed to write public tls.crt: %w", err)
	}

	if err := os.MkdirAll(keyDir, 0755); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write tls.key: %w", err)
	}

	if chownUID != -1 || chownGID != -1 {
		if err := os.Chown(keyDir, chownUID, chownGID); err != nil {
			log.Printf("Warning: failed to chown key directory %s: %v", keyDir, err)
		}
		if err := os.Chown(keyPath, chownUID, chownGID); err != nil {
			log.Printf("Warning: failed to chown key file %s: %v", keyPath, err)
		}
	}

	return nil
}

// GenerateCDISpec creates a CDI specification for the portal-egress device.
func GenerateCDISpec(caHostPath, hookHostPath string) *CDISpec {
	return &CDISpec{
		CDIVersion: "0.6.0",
		Kind:       DefaultCDIKind,
		Devices: []CDIDevice{
			{
				Name: DefaultCDIDevice,
				ContainerEdits: ContainerEdits{
					Env: []string{
						"SSL_CERT_FILE=/etc/service-portal/ca/tls.crt",
					},
					Mounts: []CDIMount{
						{
							HostPath:      caHostPath,
							ContainerPath: "/etc/service-portal/ca",
							Options:       []string{"ro", "bind"},
						},
					},
					Hooks: []CDIHook{
						{
							HookName: "createRuntime",
							Path:     hookHostPath,
						},
					},
				},
			},
		},
	}
}

// WriteCDISpec writes the CDI specification file to disk.
func WriteCDISpec(cdiDir, caHostPath, hookHostPath string) error {
	if err := os.MkdirAll(cdiDir, 0755); err != nil {
		return fmt.Errorf("failed to create CDI directory %s: %w", cdiDir, err)
	}
	spec := GenerateCDISpec(caHostPath, hookHostPath)
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal CDI spec: %w", err)
	}
	cdiFile := filepath.Join(cdiDir, "portal-egress.json")
	if err := os.WriteFile(cdiFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write CDI spec to %s: %w", cdiFile, err)
	}
	return nil
}

// DRAServer implements the Kubelet DRAPlugin gRPC service.
type DRAServer struct {
	drav1beta1.UnimplementedDRAPluginServer
	DriverName string
}

func (s *DRAServer) NodePrepareResources(ctx context.Context, req *drav1beta1.NodePrepareResourcesRequest) (*drav1beta1.NodePrepareResourcesResponse, error) {
	log.Printf("[DRA] NodePrepareResources called with %d claims", len(req.Claims))
	resp := &drav1beta1.NodePrepareResourcesResponse{
		Claims: make(map[string]*drav1beta1.NodePrepareResourceResponse),
	}

	for _, claim := range req.Claims {
		log.Printf("[DRA] Preparing claim: %s/%s (UID: %s)", claim.Namespace, claim.Name, claim.Uid)
		resp.Claims[claim.Uid] = &drav1beta1.NodePrepareResourceResponse{
			Devices: []*drav1beta1.Device{
				{
					PoolName:     "portal-egress",
					DeviceName:   "egress-0",
					CdiDeviceIds: []string{DefaultCDIKind + "=" + DefaultCDIDevice},
				},
			},
		}
	}
	return resp, nil
}

func (s *DRAServer) NodeUnprepareResources(ctx context.Context, req *drav1beta1.NodeUnprepareResourcesRequest) (*drav1beta1.NodeUnprepareResourcesResponse, error) {
	log.Printf("[DRA] NodeUnprepareResources called with %d claims", len(req.Claims))
	resp := &drav1beta1.NodeUnprepareResourcesResponse{
		Claims: make(map[string]*drav1beta1.NodeUnprepareResourceResponse),
	}
	for _, claim := range req.Claims {
		log.Printf("[DRA] Unpreparing claim: %s/%s (UID: %s)", claim.Namespace, claim.Name, claim.Uid)
		resp.Claims[claim.Uid] = &drav1beta1.NodeUnprepareResourceResponse{}
	}
	return resp, nil
}

// RegServer implements the Kubelet pluginregistration Registration gRPC service.
type RegServer struct {
	regv1.UnimplementedRegistrationServer
	DriverName   string
	DriverSocket string
}

func (s *RegServer) GetInfo(ctx context.Context, req *regv1.InfoRequest) (*regv1.PluginInfo, error) {
	log.Printf("[REG] GetInfo called by kubelet for driver %s", s.DriverName)
	return &regv1.PluginInfo{
		Type:              regv1.DRAPlugin,
		Name:              s.DriverName,
		Endpoint:          s.DriverSocket,
		SupportedVersions: []string{"v1alpha4", "v1beta1"},
	}, nil
}

func (s *RegServer) NotifyRegistrationStatus(ctx context.Context, status *regv1.RegistrationStatus) (*regv1.RegistrationStatusResponse, error) {
	log.Printf("[REG] NotifyRegistrationStatus: registered=%v, err=%s", status.PluginRegistered, status.Error)
	return &regv1.RegistrationStatusResponse{}, nil
}

// CopyFile copies a file from src to dst.
func CopyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := in.Close(); closeErr != nil {
			log.Printf("error closing src file %s: %v", src, closeErr)
		}
	}()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil {
			log.Printf("error closing dst file %s: %v", dst, closeErr)
		}
	}()

	_, err = io.Copy(out, in)
	return err
}

func main() {
	driverName := flag.String("driver-name", DefaultDriverName, "DRA driver name")
	pluginDir := flag.String("plugin-dir", "/var/lib/kubelet/plugins/"+DefaultDriverName, "Directory for driver socket")
	registrationDir := flag.String("registration-dir", "/var/lib/kubelet/plugins_registry", "Directory for kubelet plugin registration socket")
	cdiDir := flag.String("cdi-dir", "/etc/cdi", "Directory to write CDI spec JSON")
	caDir := flag.String("ca-dir", "/var/run/service-portal/ca", "Directory for CA certificate and key")
	hookSource := flag.String("hook-source", "/usr/local/bin/portal-cdi-hook", "Path to source hook binary in container")
	hookCopyTarget := flag.String("hook-copy-target", "/host/usr/local/bin/portal-cdi-hook", "Path where driver copies hook binary")
	hookHostPath := flag.String("hook-host-path", "/usr/local/bin/portal-cdi-hook", "Path on host where runc finds hook")
	hookTarget := flag.String("hook-target", "", "Deprecated: fallback for hook-copy-target and hook-host-path")
	flag.Parse()

	if *hookTarget != "" {
		if *hookCopyTarget == "/host/usr/local/bin/portal-cdi-hook" {
			*hookCopyTarget = *hookTarget
		}
		if *hookHostPath == "/usr/local/bin/portal-cdi-hook" {
			*hookHostPath = *hookTarget
		}
	}

	log.Printf("Starting DRA Portal Driver: %s", *driverName)

	// 1. Ensure CA exists
	if err := GenerateCACertificate(*caDir, *caDir, 1337, 1337); err != nil {
		log.Fatalf("failed generating CA: %v", err)
	}

	// 2. Install hook binary if source exists and differs from copy target
	if *hookSource != "" && *hookCopyTarget != "" && *hookSource != *hookCopyTarget {
		if _, err := os.Stat(*hookSource); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("Warning: error stating hook source %s: %v", *hookSource, err)
			}
		} else {
			if err := CopyFile(*hookSource, *hookCopyTarget, 0755); err != nil {
				log.Printf("Warning: failed copying hook binary from %s to %s: %v", *hookSource, *hookCopyTarget, err)
			} else {
				log.Printf("Installed hook binary to %s", *hookCopyTarget)
			}
		}
	}

	// 3. Write CDI spec using the host path where runc will find it
	if err := WriteCDISpec(*cdiDir, *caDir, *hookHostPath); err != nil {
		log.Fatalf("failed writing CDI spec: %v", err)
	}
	log.Printf("Wrote CDI spec to %s/portal-egress.json (hook path: %s)", *cdiDir, *hookHostPath)

	// 4. Start DRA Driver gRPC service
	driverSocket := filepath.Join(*pluginDir, "dra.sock")
	if err := os.MkdirAll(*pluginDir, 0755); err != nil {
		log.Fatalf("failed creating plugin dir %s: %v", *pluginDir, err)
	}
	if err := os.Remove(driverSocket); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to remove stale driver socket %s: %v", driverSocket, err)
	}

	driverLis, err := net.Listen("unix", driverSocket)
	if err != nil {
		log.Fatalf("failed listening on driver socket %s: %v", driverSocket, err)
	}
	driverGrpc := grpc.NewServer()
	draSrv := &DRAServer{DriverName: *driverName}
	drav1beta1.RegisterDRAPluginServer(driverGrpc, draSrv)

	// Also register v1alpha3.Node service descriptor so Kubelet v1alpha3/v1alpha4 clients work seamlessly
	nodeServiceDesc := grpc.ServiceDesc{
		ServiceName: "v1alpha3.Node",
		HandlerType: (*drav1beta1.DRAPluginServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "NodePrepareResources",
				Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					in := new(drav1beta1.NodePrepareResourcesRequest)
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return srv.(drav1beta1.DRAPluginServer).NodePrepareResources(ctx, in)
					}
					info := &grpc.UnaryServerInfo{
						Server:     srv,
						FullMethod: "/v1alpha3.Node/NodePrepareResources",
					}
					handler := func(ctx context.Context, req any) (any, error) {
						return srv.(drav1beta1.DRAPluginServer).NodePrepareResources(ctx, req.(*drav1beta1.NodePrepareResourcesRequest))
					}
					return interceptor(ctx, in, info, handler)
				},
			},
			{
				MethodName: "NodeUnprepareResources",
				Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					in := new(drav1beta1.NodeUnprepareResourcesRequest)
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return srv.(drav1beta1.DRAPluginServer).NodeUnprepareResources(ctx, in)
					}
					info := &grpc.UnaryServerInfo{
						Server:     srv,
						FullMethod: "/v1alpha3.Node/NodeUnprepareResources",
					}
					handler := func(ctx context.Context, req any) (any, error) {
						return srv.(drav1beta1.DRAPluginServer).NodeUnprepareResources(ctx, req.(*drav1beta1.NodeUnprepareResourcesRequest))
					}
					return interceptor(ctx, in, info, handler)
				},
			},
		},
		Streams: []grpc.StreamDesc{},
	}
	driverGrpc.RegisterService(&nodeServiceDesc, draSrv)

	go func() {
		if err := driverGrpc.Serve(driverLis); err != nil {
			log.Printf("DRA driver server stopped: %v", err)
		}
	}()
	log.Printf("DRA driver gRPC server listening on %s", driverSocket)

	// 5. Start Plugin Registration service
	regSocket := filepath.Join(*registrationDir, *driverName+"-reg.sock")
	if err := os.MkdirAll(*registrationDir, 0755); err != nil {
		log.Fatalf("failed creating registration dir %s: %v", *registrationDir, err)
	}
	if err := os.Remove(regSocket); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to remove stale reg socket %s: %v", regSocket, err)
	}

	regLis, err := net.Listen("unix", regSocket)
	if err != nil {
		log.Fatalf("failed listening on registration socket %s: %v", regSocket, err)
	}
	regGrpc := grpc.NewServer()
	regSrv := &RegServer{DriverName: *driverName, DriverSocket: driverSocket}
	regv1.RegisterRegistrationServer(regGrpc, regSrv)

	go func() {
		if err := regGrpc.Serve(regLis); err != nil {
			log.Printf("Registration server stopped: %v", err)
		}
	}()
	log.Printf("Registration gRPC server listening on %s", regSocket)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down DRA Portal Driver...")
	driverGrpc.GracefulStop()
	regGrpc.GracefulStop()
	if err := os.Remove(regSocket); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed removing reg socket %s: %v", regSocket, err)
	}
	if err := os.Remove(driverSocket); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed removing driver socket %s: %v", driverSocket, err)
	}
	log.Println("Driver shutdown complete.")
}
