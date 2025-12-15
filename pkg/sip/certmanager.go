// Package sip provides SIP server functionality using sipgo
package sip

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/btafoya/gosip/internal/config"
	"github.com/caddyserver/certmagic"
	"github.com/libdns/cloudflare"
)

// CertManager handles TLS certificate lifecycle management
type CertManager struct {
	config    *config.TLSConfig
	dataDir   string
	tlsConfig *tls.Config
	magic     *certmagic.Config
	mu        sync.RWMutex

	// Certificate info for status reporting
	certExpiry  time.Time
	certIssuer  string
	lastRenewal time.Time
}

// CertStatus represents the current certificate status
type CertStatus struct {
	Enabled     bool      `json:"enabled"`
	CertMode    string    `json:"cert_mode"`
	Domain      string    `json:"domain,omitempty"`
	Domains     []string  `json:"domains,omitempty"`
	CertExpiry  time.Time `json:"cert_expiry,omitempty"`
	CertIssuer  string    `json:"cert_issuer,omitempty"`
	AutoRenewal bool      `json:"auto_renewal"`
	LastRenewal time.Time `json:"last_renewal,omitempty"`
	NextRenewal time.Time `json:"next_renewal,omitempty"`
	Valid       bool      `json:"valid"`
	Error       string    `json:"error,omitempty"`
}

// NewCertManager creates a new certificate manager
func NewCertManager(cfg *config.TLSConfig, dataDir string) (*CertManager, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}

	cm := &CertManager{
		config:  cfg,
		dataDir: dataDir,
	}

	var err error
	if cfg.CertMode == "manual" {
		err = cm.initManual()
	} else {
		err = cm.initACME()
	}

	if err != nil {
		return nil, err
	}

	return cm, nil
}

// initManual loads certificates from files
func (cm *CertManager) initManual() error {
	if cm.config.CertFile == "" || cm.config.KeyFile == "" {
		return fmt.Errorf("certificate and key file paths required for manual mode")
	}

	cert, err := tls.LoadX509KeyPair(cm.config.CertFile, cm.config.KeyFile)
	if err != nil {
		return fmt.Errorf("load certificate: %w", err)
	}

	// Parse certificate to extract info
	if len(cert.Certificate) > 0 {
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err == nil {
			cm.certExpiry = x509Cert.NotAfter
			cm.certIssuer = x509Cert.Issuer.CommonName
		}
	}

	// Build TLS config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   cm.getTLSMinVersion(),
	}

	// Load CA certificate if provided
	if cm.config.CAFile != "" {
		caCert, err := os.ReadFile(cm.config.CAFile)
		if err != nil {
			return fmt.Errorf("load CA certificate: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
		tlsConfig.ClientCAs = caCertPool
	}

	// Configure client authentication
	tlsConfig.ClientAuth = cm.getClientAuth()

	cm.tlsConfig = tlsConfig
	slog.Info("TLS initialized with manual certificates",
		"cert_file", cm.config.CertFile,
		"expiry", cm.certExpiry.Format(time.RFC3339),
	)

	return nil
}

// initACME sets up automatic certificate management with Let's Encrypt
func (cm *CertManager) initACME() error {
	if cm.config.ACMEEmail == "" {
		return fmt.Errorf("ACME email required for automatic certificate management")
	}
	if cm.config.ACMEDomain == "" {
		return fmt.Errorf("ACME domain required for automatic certificate management")
	}

	// Configure storage path
	certsPath := filepath.Join(cm.dataDir, "certs")
	if err := os.MkdirAll(certsPath, 0700); err != nil {
		return fmt.Errorf("create certs directory: %w", err)
	}

	// Configure file storage
	certmagic.Default.Storage = &certmagic.FileStorage{
		Path: certsPath,
	}

	// Configure DNS-01 challenge with Cloudflare
	if cm.config.CloudflareAPIToken != "" {
		cfProvider := &cloudflare.Provider{
			APIToken: cm.config.CloudflareAPIToken,
		}

		certmagic.DefaultACME.DNS01Solver = &certmagic.DNS01Solver{
			DNSProvider: cfProvider,
		}
		slog.Info("Configured Cloudflare DNS-01 challenge for ACME")
	}

	// Configure ACME settings
	certmagic.DefaultACME.Agreed = true
	certmagic.DefaultACME.Email = cm.config.ACMEEmail

	// Set CA based on configuration
	if cm.config.ACMECA == "production" {
		certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA
		slog.Info("Using Let's Encrypt production CA")
	} else {
		certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA
		slog.Info("Using Let's Encrypt staging CA")
	}

	// Create CertMagic config
	cm.magic = certmagic.NewDefault()

	// Build domain list
	domains := []string{cm.config.ACMEDomain}
	domains = append(domains, cm.config.ACMEDomains...)

	// Set up event handler for certificate status updates
	cm.magic.OnEvent = func(ctx context.Context, event string, data map[string]any) error {
		switch event {
		case "cert_obtained", "cert_renewed":
			cm.mu.Lock()
			cm.lastRenewal = time.Now()
			cm.mu.Unlock()
			slog.Info("Certificate obtained/renewed", "event", event, "data", data)
		case "cert_failed":
			slog.Error("Certificate operation failed", "event", event, "data", data)
		}
		return nil
	}

	// Obtain certificates asynchronously (don't block startup)
	ctx := context.Background()
	if err := cm.magic.ManageAsync(ctx, domains); err != nil {
		return fmt.Errorf("certmagic manage: %w", err)
	}

	// Get TLS config from CertMagic
	cm.tlsConfig = cm.magic.TLSConfig()
	cm.tlsConfig.MinVersion = cm.getTLSMinVersion()
	cm.tlsConfig.ClientAuth = cm.getClientAuth()

	slog.Info("TLS initialized with ACME",
		"email", cm.config.ACMEEmail,
		"domain", cm.config.ACMEDomain,
		"ca", cm.config.ACMECA,
	)

	return nil
}

// getTLSMinVersion returns the tls.Config minimum version constant
func (cm *CertManager) getTLSMinVersion() uint16 {
	switch cm.config.MinVersion {
	case "1.3":
		return tls.VersionTLS13
	default:
		return tls.VersionTLS12
	}
}

// getClientAuth returns the tls.ClientAuthType constant
func (cm *CertManager) getClientAuth() tls.ClientAuthType {
	switch cm.config.ClientAuth {
	case "request":
		return tls.RequestClientCert
	case "require":
		return tls.RequireAndVerifyClientCert
	default:
		return tls.NoClientCert
	}
}

// GetTLSConfig returns the current TLS configuration
func (cm *CertManager) GetTLSConfig() *tls.Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.tlsConfig
}

// GetStatus returns the current certificate status
func (cm *CertManager) GetStatus() CertStatus {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	status := CertStatus{
		Enabled:     cm.config.Enabled,
		CertMode:    cm.config.CertMode,
		Domain:      cm.config.ACMEDomain,
		Domains:     cm.config.ACMEDomains,
		CertExpiry:  cm.certExpiry,
		CertIssuer:  cm.certIssuer,
		AutoRenewal: cm.config.CertMode == "acme",
		LastRenewal: cm.lastRenewal,
	}

	// Calculate next renewal (typically 30 days before expiry)
	if !cm.certExpiry.IsZero() {
		status.NextRenewal = cm.certExpiry.Add(-30 * 24 * time.Hour)
		status.Valid = time.Now().Before(cm.certExpiry)
	}

	return status
}

// ForceRenewal triggers immediate certificate renewal (ACME mode only)
func (cm *CertManager) ForceRenewal(ctx context.Context) error {
	if cm.config.CertMode != "acme" {
		return fmt.Errorf("force renewal only available in ACME mode")
	}

	if cm.magic == nil {
		return fmt.Errorf("ACME not initialized")
	}

	domains := []string{cm.config.ACMEDomain}
	domains = append(domains, cm.config.ACMEDomains...)

	// Force renewal by managing domains again
	if err := cm.magic.ManageSync(ctx, domains); err != nil {
		return fmt.Errorf("renewal failed: %w", err)
	}

	cm.mu.Lock()
	cm.lastRenewal = time.Now()
	cm.mu.Unlock()

	slog.Info("Certificate renewal completed")
	return nil
}

// ReloadCertificates reloads certificates from files (manual mode only)
func (cm *CertManager) ReloadCertificates() error {
	if cm.config.CertMode != "manual" {
		return fmt.Errorf("reload only available in manual mode")
	}

	return cm.initManual()
}

// Close cleans up certificate manager resources
func (cm *CertManager) Close() error {
	// CertMagic handles cleanup automatically
	return nil
}

// GenerateTLSConfig creates a TLS configuration from certificate files
// This is a helper function for manual certificate setup
func GenerateTLSConfig(certFile, keyFile string, rootPems []byte) (*tls.Config, error) {
	roots := x509.NewCertPool()
	if rootPems != nil {
		ok := roots.AppendCertsFromPEM(rootPems)
		if !ok {
			return nil, fmt.Errorf("failed to parse root certificate")
		}
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("fail to load cert: %w", err)
	}

	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      roots,
		MinVersion:   tls.VersionTLS12,
	}

	return conf, nil
}
