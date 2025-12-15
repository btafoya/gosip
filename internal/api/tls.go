package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/pkg/sip"
)

// TLSHandler handles TLS configuration and status API endpoints
type TLSHandler struct {
	deps *Dependencies
}

// NewTLSHandler creates a new TLSHandler
func NewTLSHandler(deps *Dependencies) *TLSHandler {
	return &TLSHandler{deps: deps}
}

// TLSStatusResponse represents the TLS status API response
type TLSStatusResponse struct {
	Enabled     bool      `json:"enabled"`
	CertMode    string    `json:"cert_mode"`
	Domain      string    `json:"domain,omitempty"`
	Domains     []string  `json:"domains,omitempty"`
	Port        int       `json:"port"`
	WSSPort     int       `json:"wss_port"`
	CertExpiry  time.Time `json:"cert_expiry,omitempty"`
	CertIssuer  string    `json:"cert_issuer,omitempty"`
	AutoRenewal bool      `json:"auto_renewal"`
	LastRenewal time.Time `json:"last_renewal,omitempty"`
	NextRenewal time.Time `json:"next_renewal,omitempty"`
	Valid       bool      `json:"valid"`
	Error       string    `json:"error,omitempty"`
}

// GetStatus returns the current TLS configuration and certificate status
func (h *TLSHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get TLS configuration from database
	enabled := h.deps.DB.Config.GetWithDefault(ctx, db.ConfigKeyTLSEnabled, "false") == "true"
	certMode := h.deps.DB.Config.GetWithDefault(ctx, db.ConfigKeyTLSCertMode, "acme")
	domain := h.deps.DB.Config.GetWithDefault(ctx, db.ConfigKeyACMEDomain, "")
	domainsStr := h.deps.DB.Config.GetWithDefault(ctx, db.ConfigKeyACMEDomains, "")
	portStr := h.deps.DB.Config.GetWithDefault(ctx, db.ConfigKeyTLSPort, "5061")
	wssPortStr := h.deps.DB.Config.GetWithDefault(ctx, db.ConfigKeyTLSWSSPort, "5081")

	port, _ := strconv.Atoi(portStr)
	wssPort, _ := strconv.Atoi(wssPortStr)

	var domains []string
	if domainsStr != "" {
		domains = strings.Split(domainsStr, ",")
	}

	response := TLSStatusResponse{
		Enabled:     enabled,
		CertMode:    certMode,
		Domain:      domain,
		Domains:     domains,
		Port:        port,
		WSSPort:     wssPort,
		AutoRenewal: certMode == "acme",
	}

	// Get live certificate status from SIP server if available
	if h.deps.SIP != nil {
		status := h.deps.SIP.GetTLSStatus()
		if status != nil {
			response.CertExpiry = status.CertExpiry
			response.CertIssuer = status.CertIssuer
			response.LastRenewal = status.LastRenewal
			response.NextRenewal = status.NextRenewal
			response.Valid = status.Valid
			response.Error = status.Error
		}
	}

	WriteJSON(w, http.StatusOK, response)
}

// TLSConfigRequest represents a request to update TLS configuration
type TLSConfigRequest struct {
	Enabled            *bool    `json:"enabled,omitempty"`
	CertMode           string   `json:"cert_mode,omitempty"`
	Port               *int     `json:"port,omitempty"`
	WSSPort            *int     `json:"wss_port,omitempty"`
	CertFile           string   `json:"cert_file,omitempty"`
	KeyFile            string   `json:"key_file,omitempty"`
	CAFile             string   `json:"ca_file,omitempty"`
	ACMEEmail          string   `json:"acme_email,omitempty"`
	ACMEDomain         string   `json:"acme_domain,omitempty"`
	ACMEDomains        []string `json:"acme_domains,omitempty"`
	ACMECA             string   `json:"acme_ca,omitempty"`
	CloudflareAPIToken string   `json:"cloudflare_api_token,omitempty"`
	MinVersion         string   `json:"min_version,omitempty"`
	ClientAuth         string   `json:"client_auth,omitempty"`
}

// UpdateConfig updates TLS configuration
func (h *TLSHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req TLSConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	ctx := r.Context()

	// Update TLS enabled
	if req.Enabled != nil {
		value := "false"
		if *req.Enabled {
			value = "true"
		}
		h.deps.DB.Config.Set(ctx, db.ConfigKeyTLSEnabled, value)
	}

	// Update certificate mode
	if req.CertMode != "" {
		if req.CertMode != "manual" && req.CertMode != "acme" {
			WriteValidationError(w, "Invalid cert_mode. Must be 'manual' or 'acme'", []FieldError{
				{Field: "cert_mode", Message: "Must be 'manual' or 'acme'"},
			})
			return
		}
		h.deps.DB.Config.Set(ctx, db.ConfigKeyTLSCertMode, req.CertMode)
	}

	// Update ports
	if req.Port != nil && *req.Port > 0 {
		h.deps.DB.Config.Set(ctx, db.ConfigKeyTLSPort, fmt.Sprintf("%d", *req.Port))
	}
	if req.WSSPort != nil && *req.WSSPort > 0 {
		h.deps.DB.Config.Set(ctx, db.ConfigKeyTLSWSSPort, fmt.Sprintf("%d", *req.WSSPort))
	}

	// Update manual certificate paths
	if req.CertFile != "" {
		h.deps.DB.Config.Set(ctx, db.ConfigKeyTLSCertFile, req.CertFile)
	}
	if req.KeyFile != "" {
		h.deps.DB.Config.Set(ctx, db.ConfigKeyTLSKeyFile, req.KeyFile)
	}
	if req.CAFile != "" {
		h.deps.DB.Config.Set(ctx, db.ConfigKeyTLSCAFile, req.CAFile)
	}

	// Update ACME settings
	if req.ACMEEmail != "" {
		h.deps.DB.Config.Set(ctx, db.ConfigKeyACMEEmail, req.ACMEEmail)
	}
	if req.ACMEDomain != "" {
		h.deps.DB.Config.Set(ctx, db.ConfigKeyACMEDomain, req.ACMEDomain)
	}
	if len(req.ACMEDomains) > 0 {
		h.deps.DB.Config.Set(ctx, db.ConfigKeyACMEDomains, strings.Join(req.ACMEDomains, ","))
	}
	if req.ACMECA != "" {
		if req.ACMECA != "staging" && req.ACMECA != "production" {
			WriteValidationError(w, "Invalid acme_ca. Must be 'staging' or 'production'", []FieldError{
				{Field: "acme_ca", Message: "Must be 'staging' or 'production'"},
			})
			return
		}
		h.deps.DB.Config.Set(ctx, db.ConfigKeyACMECA, req.ACMECA)
	}

	// Update Cloudflare API token
	if req.CloudflareAPIToken != "" {
		h.deps.DB.Config.Set(ctx, db.ConfigKeyCloudflareAPIToken, req.CloudflareAPIToken)
	}

	// Update TLS version
	if req.MinVersion != "" {
		if req.MinVersion != "1.2" && req.MinVersion != "1.3" {
			WriteValidationError(w, "Invalid min_version. Must be '1.2' or '1.3'", []FieldError{
				{Field: "min_version", Message: "Must be '1.2' or '1.3'"},
			})
			return
		}
		h.deps.DB.Config.Set(ctx, db.ConfigKeyTLSMinVersion, req.MinVersion)
	}

	// Update client authentication
	if req.ClientAuth != "" {
		if req.ClientAuth != "none" && req.ClientAuth != "request" && req.ClientAuth != "require" {
			WriteValidationError(w, "Invalid client_auth. Must be 'none', 'request', or 'require'", []FieldError{
				{Field: "client_auth", Message: "Must be 'none', 'request', or 'require'"},
			})
			return
		}
		h.deps.DB.Config.Set(ctx, db.ConfigKeyTLSClientAuth, req.ClientAuth)
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"message": "TLS configuration updated. Restart the server to apply changes.",
	})
}

// ForceRenewal triggers immediate certificate renewal (ACME mode only)
func (h *TLSHandler) ForceRenewal(w http.ResponseWriter, r *http.Request) {
	if h.deps.SIP == nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeInternal, "SIP server not available", nil)
		return
	}

	if !h.deps.SIP.IsTLSEnabled() {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "TLS is not enabled", nil)
		return
	}

	if err := h.deps.SIP.ForceRenewal(r.Context()); err != nil {
		WriteError(w, http.StatusInternalServerError, ErrCodeInternal, fmt.Sprintf("Certificate renewal failed: %v", err), nil)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Certificate renewal initiated successfully",
	})
}

// ReloadCertificates reloads certificates from files (manual mode only)
func (h *TLSHandler) ReloadCertificates(w http.ResponseWriter, r *http.Request) {
	if h.deps.SIP == nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeInternal, "SIP server not available", nil)
		return
	}

	if !h.deps.SIP.IsTLSEnabled() {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "TLS is not enabled", nil)
		return
	}

	if err := h.deps.SIP.ReloadCertificates(); err != nil {
		WriteError(w, http.StatusInternalServerError, ErrCodeInternal, fmt.Sprintf("Certificate reload failed: %v", err), nil)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Certificates reloaded successfully",
	})
}

// SRTPStatusResponse represents the SRTP status API response
type SRTPStatusResponse struct {
	Enabled bool   `json:"enabled"`
	Profile string `json:"profile"`
}

// GetSRTPStatus returns the current SRTP configuration
func (h *TLSHandler) GetSRTPStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	enabled := h.deps.DB.Config.GetWithDefault(ctx, db.ConfigKeySRTPEnabled, "false") == "true"
	profile := h.deps.DB.Config.GetWithDefault(ctx, db.ConfigKeySRTPProfile, "AES_CM_128_HMAC_SHA1_80")

	WriteJSON(w, http.StatusOK, SRTPStatusResponse{
		Enabled: enabled,
		Profile: profile,
	})
}

// SRTPConfigRequest represents a request to update SRTP configuration
type SRTPConfigRequest struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Profile string `json:"profile,omitempty"`
}

// UpdateSRTPConfig updates SRTP configuration
func (h *TLSHandler) UpdateSRTPConfig(w http.ResponseWriter, r *http.Request) {
	var req SRTPConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	ctx := r.Context()

	if req.Enabled != nil {
		value := "false"
		if *req.Enabled {
			value = "true"
		}
		h.deps.DB.Config.Set(ctx, db.ConfigKeySRTPEnabled, value)
	}

	if req.Profile != "" {
		validProfiles := []string{"AES_CM_128_HMAC_SHA1_80", "AES_CM_128_HMAC_SHA1_32", "AEAD_AES_128_GCM", "AEAD_AES_256_GCM"}
		valid := false
		for _, p := range validProfiles {
			if req.Profile == p {
				valid = true
				break
			}
		}
		if !valid {
			WriteValidationError(w, "Invalid SRTP profile", []FieldError{
				{Field: "profile", Message: "Must be one of: AES_CM_128_HMAC_SHA1_80, AES_CM_128_HMAC_SHA1_32, AEAD_AES_128_GCM, AEAD_AES_256_GCM"},
			})
			return
		}
		h.deps.DB.Config.Set(ctx, db.ConfigKeySRTPProfile, req.Profile)
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"message": "SRTP configuration updated. Restart the server to apply changes.",
	})
}

// GetCertificateInfo returns detailed certificate information
func (h *TLSHandler) GetCertificateInfo(w http.ResponseWriter, r *http.Request) {
	if h.deps.SIP == nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeInternal, "SIP server not available", nil)
		return
	}

	certMgr := h.deps.SIP.GetCertManager()
	if certMgr == nil {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "TLS is not enabled", nil)
		return
	}

	status := certMgr.GetStatus()
	WriteJSON(w, http.StatusOK, sip.CertStatus(status))
}
