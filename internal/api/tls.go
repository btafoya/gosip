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

// ZRTPStatusResponse represents the ZRTP status API response
type ZRTPStatusResponse struct {
	Enabled         bool   `json:"enabled"`
	Mode            string `json:"mode"`
	CacheExpiryDays int    `json:"cache_expiry_days"`
	ActiveSessions  int    `json:"active_sessions"`
	CachedPeers     int    `json:"cached_peers"`
}

// GetZRTPStatus returns the current ZRTP configuration and status
func (h *TLSHandler) GetZRTPStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	enabled := h.deps.DB.Config.GetWithDefault(ctx, "zrtp.enabled", "false") == "true"
	mode := h.deps.DB.Config.GetWithDefault(ctx, "zrtp.mode", "optional")
	cacheExpiryStr := h.deps.DB.Config.GetWithDefault(ctx, "zrtp.cache_expiry_days", "90")
	cacheExpiry, _ := strconv.Atoi(cacheExpiryStr)

	response := ZRTPStatusResponse{
		Enabled:         enabled,
		Mode:            mode,
		CacheExpiryDays: cacheExpiry,
	}

	// Get live statistics from SIP server if available
	if h.deps.SIP != nil && h.deps.SIP.IsZRTPEnabled() {
		stats := h.deps.SIP.GetZRTPStats()
		if activeSessions, ok := stats["active_sessions"].(int); ok {
			response.ActiveSessions = activeSessions
		}
		if cachedPeers, ok := stats["cached_peers"].(int); ok {
			response.CachedPeers = cachedPeers
		}
	}

	WriteJSON(w, http.StatusOK, response)
}

// ZRTPConfigRequest represents a request to update ZRTP configuration
type ZRTPConfigRequest struct {
	Enabled         *bool  `json:"enabled,omitempty"`
	Mode            string `json:"mode,omitempty"`
	CacheExpiryDays *int   `json:"cache_expiry_days,omitempty"`
}

// UpdateZRTPConfig updates ZRTP configuration
func (h *TLSHandler) UpdateZRTPConfig(w http.ResponseWriter, r *http.Request) {
	var req ZRTPConfigRequest
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
		h.deps.DB.Config.Set(ctx, "zrtp.enabled", value)
	}

	if req.Mode != "" {
		if req.Mode != "optional" && req.Mode != "required" && req.Mode != "disabled" {
			WriteValidationError(w, "Invalid ZRTP mode", []FieldError{
				{Field: "mode", Message: "Must be 'optional', 'required', or 'disabled'"},
			})
			return
		}
		h.deps.DB.Config.Set(ctx, "zrtp.mode", req.Mode)
	}

	if req.CacheExpiryDays != nil {
		if *req.CacheExpiryDays < 1 || *req.CacheExpiryDays > 365 {
			WriteValidationError(w, "Invalid cache expiry days", []FieldError{
				{Field: "cache_expiry_days", Message: "Must be between 1 and 365"},
			})
			return
		}
		h.deps.DB.Config.Set(ctx, "zrtp.cache_expiry_days", fmt.Sprintf("%d", *req.CacheExpiryDays))
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"message": "ZRTP configuration updated. Restart the server to apply changes.",
	})
}

// ZRTPSessionInfo represents information about an active ZRTP session
type ZRTPSessionInfo struct {
	CallID    string    `json:"call_id"`
	State     string    `json:"state"`
	SAS       string    `json:"sas,omitempty"`
	IsCached  bool      `json:"is_cached"`
	StartedAt time.Time `json:"started_at"`
	SecuredAt time.Time `json:"secured_at,omitempty"`
}

// GetZRTPSessions returns all active ZRTP sessions
func (h *TLSHandler) GetZRTPSessions(w http.ResponseWriter, r *http.Request) {
	if h.deps.SIP == nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeInternal, "SIP server not available", nil)
		return
	}

	zrtpMgr := h.deps.SIP.GetZRTPManager()
	if zrtpMgr == nil {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "ZRTP is not enabled", nil)
		return
	}

	// Get all active sessions from the session manager
	sessions := h.deps.SIP.GetSessions()
	if sessions == nil {
		WriteJSON(w, http.StatusOK, []ZRTPSessionInfo{})
		return
	}

	var zrtpSessions []ZRTPSessionInfo
	for _, callID := range sessions.GetAllCallIDs() {
		zrtpSession, ok := zrtpMgr.GetSession(callID)
		if ok && zrtpSession != nil {
			info := ZRTPSessionInfo{
				CallID:    callID,
				State:     string(zrtpSession.State),
				SAS:       zrtpSession.SAS,
				IsCached:  zrtpSession.IsCached,
				StartedAt: zrtpSession.StartedAt,
				SecuredAt: zrtpSession.SecuredAt,
			}
			zrtpSessions = append(zrtpSessions, info)
		}
	}

	WriteJSON(w, http.StatusOK, zrtpSessions)
}

// ZRTPSASRequest represents a request to verify SAS for a call
type ZRTPSASRequest struct {
	Verified bool `json:"verified"`
}

// GetZRTPSAS returns the SAS for a specific call
func (h *TLSHandler) GetZRTPSAS(w http.ResponseWriter, r *http.Request) {
	callID := r.URL.Query().Get("call_id")
	if callID == "" {
		WriteValidationError(w, "Missing call_id parameter", nil)
		return
	}

	if h.deps.SIP == nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeInternal, "SIP server not available", nil)
		return
	}

	sas, err := h.deps.SIP.GetZRTPSAS(callID)
	if err != nil {
		WriteError(w, http.StatusNotFound, ErrCodeNotFound, fmt.Sprintf("ZRTP session not found: %v", err), nil)
		return
	}

	isSecured := h.deps.SIP.IsCallZRTPSecured(callID)

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"call_id":   callID,
		"sas":       sas,
		"is_secure": isSecured,
	})
}

// VerifyZRTPSAS allows manual verification of SAS through the API
func (h *TLSHandler) VerifyZRTPSAS(w http.ResponseWriter, r *http.Request) {
	callID := r.URL.Query().Get("call_id")
	if callID == "" {
		WriteValidationError(w, "Missing call_id parameter", nil)
		return
	}

	var req ZRTPSASRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if h.deps.SIP == nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeInternal, "SIP server not available", nil)
		return
	}

	zrtpMgr := h.deps.SIP.GetZRTPManager()
	if zrtpMgr == nil {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "ZRTP is not enabled", nil)
		return
	}

	session, ok := zrtpMgr.GetSession(callID)
	if !ok {
		WriteError(w, http.StatusNotFound, ErrCodeNotFound, "ZRTP session not found", nil)
		return
	}

	if req.Verified {
		// Mark session as secured
		session.State = sip.ZRTPStateSecured
		session.SecuredAt = time.Now()
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"call_id":   callID,
			"message":   "SAS verified - call is now secured",
			"is_secure": true,
		})
	} else {
		// Mark as failed - potential MITM
		session.State = sip.ZRTPStateFailed
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"call_id":   callID,
			"message":   "SAS verification failed - potential security issue",
			"is_secure": false,
			"warning":   "SAS mismatch may indicate a man-in-the-middle attack",
		})
	}
}

// EncryptionStatusResponse represents the comprehensive encryption status
type EncryptionStatusResponse struct {
	TLS struct {
		Enabled             bool      `json:"enabled"`
		UnencryptedDisabled bool      `json:"unencrypted_disabled"`
		CertMode            string    `json:"cert_mode,omitempty"`
		CertValid           bool      `json:"cert_valid,omitempty"`
		CertExpiry          time.Time `json:"cert_expiry,omitempty"`
	} `json:"tls"`
	SRTP struct {
		Enabled bool   `json:"enabled"`
		Profile string `json:"profile"`
	} `json:"srtp"`
	ZRTP struct {
		Enabled        bool   `json:"enabled"`
		Mode           string `json:"mode"`
		ActiveSessions int    `json:"active_sessions"`
		CachedPeers    int    `json:"cached_peers"`
	} `json:"zrtp"`
	OverallSecurityLevel string `json:"overall_security_level"`
}

// GetEncryptionStatus returns comprehensive encryption status
func (h *TLSHandler) GetEncryptionStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var response EncryptionStatusResponse

	// TLS status
	response.TLS.Enabled = h.deps.DB.Config.GetWithDefault(ctx, db.ConfigKeyTLSEnabled, "false") == "true"
	response.TLS.UnencryptedDisabled = h.deps.DB.Config.GetWithDefault(ctx, "tls.disable_unencrypted", "false") == "true"

	if h.deps.SIP != nil && h.deps.SIP.IsTLSEnabled() {
		tlsStatus := h.deps.SIP.GetTLSStatus()
		if tlsStatus != nil {
			response.TLS.CertMode = tlsStatus.CertMode
			response.TLS.CertValid = tlsStatus.Valid
			response.TLS.CertExpiry = tlsStatus.CertExpiry
		}
	}

	// SRTP status
	response.SRTP.Enabled = h.deps.DB.Config.GetWithDefault(ctx, db.ConfigKeySRTPEnabled, "false") == "true"
	response.SRTP.Profile = h.deps.DB.Config.GetWithDefault(ctx, db.ConfigKeySRTPProfile, "AES_CM_128_HMAC_SHA1_80")

	// ZRTP status
	response.ZRTP.Enabled = h.deps.DB.Config.GetWithDefault(ctx, "zrtp.enabled", "false") == "true"
	response.ZRTP.Mode = h.deps.DB.Config.GetWithDefault(ctx, "zrtp.mode", "optional")

	if h.deps.SIP != nil && h.deps.SIP.IsZRTPEnabled() {
		stats := h.deps.SIP.GetZRTPStats()
		if activeSessions, ok := stats["active_sessions"].(int); ok {
			response.ZRTP.ActiveSessions = activeSessions
		}
		if cachedPeers, ok := stats["cached_peers"].(int); ok {
			response.ZRTP.CachedPeers = cachedPeers
		}
	}

	// Calculate overall security level
	response.OverallSecurityLevel = calculateSecurityLevel(
		response.TLS.Enabled,
		response.TLS.UnencryptedDisabled,
		response.SRTP.Enabled,
		response.ZRTP.Enabled,
		response.ZRTP.Mode,
	)

	WriteJSON(w, http.StatusOK, response)
}

// calculateSecurityLevel determines the overall security level based on configuration
func calculateSecurityLevel(tlsEnabled, unencryptedDisabled, srtpEnabled, zrtpEnabled bool, zrtpMode string) string {
	if !tlsEnabled && !srtpEnabled && !zrtpEnabled {
		return "none"
	}

	if tlsEnabled && unencryptedDisabled && srtpEnabled && zrtpEnabled && zrtpMode == "required" {
		return "maximum"
	}

	if tlsEnabled && srtpEnabled && zrtpEnabled {
		return "high"
	}

	if tlsEnabled && srtpEnabled {
		return "medium"
	}

	if tlsEnabled || srtpEnabled {
		return "basic"
	}

	return "partial"
}

// TrunkTLSStatusResponse represents the TLS status of Twilio trunks
type TrunkTLSStatusResponse struct {
	Trunks []TrunkTLSInfo `json:"trunks"`
}

// TrunkTLSInfo represents TLS info for a single trunk
type TrunkTLSInfo struct {
	TrunkSID         string                   `json:"trunk_sid"`
	FriendlyName     string                   `json:"friendly_name"`
	SecureMode       bool                     `json:"secure_mode"`
	AllSecure        bool                     `json:"all_secure"`
	InsecureURLCount int                      `json:"insecure_url_count"`
	OriginationURLs  []OriginationURLInfo     `json:"origination_urls"`
}

// OriginationURLInfo represents info about an origination URL
type OriginationURLInfo struct {
	SID          string `json:"sid"`
	SipURL       string `json:"sip_url"`
	FriendlyName string `json:"friendly_name"`
	IsSecure     bool   `json:"is_secure"`
	Enabled      bool   `json:"enabled"`
}

// GetTrunkTLSStatus returns TLS status for all Twilio trunks
func (h *TLSHandler) GetTrunkTLSStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.deps.Twilio == nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeInternal, "Twilio client not available", nil)
		return
	}

	trunks, err := h.deps.Twilio.ListSIPTrunks(ctx)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrCodeInternal, "Failed to list SIP trunks: "+err.Error(), nil)
		return
	}

	var response TrunkTLSStatusResponse
	response.Trunks = make([]TrunkTLSInfo, 0, len(trunks))

	for _, trunk := range trunks {
		status, err := h.deps.Twilio.GetTrunkTLSStatus(ctx, trunk.SID)
		if err != nil {
			// Log but continue with other trunks
			continue
		}

		info := TrunkTLSInfo{
			TrunkSID:         status.TrunkSID,
			FriendlyName:     status.FriendlyName,
			SecureMode:       status.SecureMode,
			AllSecure:        status.AllSecure,
			InsecureURLCount: status.InsecureURLCount,
			OriginationURLs:  make([]OriginationURLInfo, 0, len(status.OriginationURLs)),
		}

		for _, url := range status.OriginationURLs {
			isSecure := strings.HasPrefix(url.SipURL, "sips:") || strings.Contains(url.SipURL, ":5061")
			info.OriginationURLs = append(info.OriginationURLs, OriginationURLInfo{
				SID:          url.SID,
				SipURL:       url.SipURL,
				FriendlyName: url.FriendlyName,
				IsSecure:     isSecure,
				Enabled:      url.Enabled,
			})
		}

		response.Trunks = append(response.Trunks, info)
	}

	WriteJSON(w, http.StatusOK, response)
}

// EnableTrunkTLSRequest represents a request to enable TLS on a trunk
type EnableTrunkTLSRequest struct {
	TrunkSID           string `json:"trunk_sid"`
	MigrateOrigination bool   `json:"migrate_origination"`
}

// EnableTrunkTLS enables TLS on a Twilio trunk
func (h *TLSHandler) EnableTrunkTLS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req EnableTrunkTLSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.TrunkSID == "" {
		WriteValidationError(w, "trunk_sid is required", nil)
		return
	}

	if h.deps.Twilio == nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeInternal, "Twilio client not available", nil)
		return
	}

	if req.MigrateOrigination {
		// Full migration: enable TLS and update all origination URLs
		if err := h.deps.Twilio.EnsureTrunkFullySecure(ctx, req.TrunkSID); err != nil {
			WriteError(w, http.StatusInternalServerError, ErrCodeInternal, "Failed to enable TLS: "+err.Error(), nil)
			return
		}
	} else {
		// Just enable secure mode on trunk
		if err := h.deps.Twilio.EnableTLSForTrunk(ctx, req.TrunkSID); err != nil {
			WriteError(w, http.StatusInternalServerError, ErrCodeInternal, "Failed to enable TLS: "+err.Error(), nil)
			return
		}
	}

	// Get updated status
	status, err := h.deps.Twilio.GetTrunkTLSStatus(ctx, req.TrunkSID)
	if err != nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"message": "TLS enabled successfully",
		})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "TLS enabled successfully",
		"secure_mode":  status.SecureMode,
		"all_secure":   status.AllSecure,
		"insecure_urls": status.InsecureURLCount,
	})
}

// MigrateOriginationURLsRequest represents a request to migrate origination URLs to TLS
type MigrateOriginationURLsRequest struct {
	TrunkSID string `json:"trunk_sid"`
}

// MigrateTrunkOrigination migrates all origination URLs to TLS
func (h *TLSHandler) MigrateTrunkOrigination(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req MigrateOriginationURLsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.TrunkSID == "" {
		WriteValidationError(w, "trunk_sid is required", nil)
		return
	}

	if h.deps.Twilio == nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeInternal, "Twilio client not available", nil)
		return
	}

	if err := h.deps.Twilio.MigrateToSecureOrigination(ctx, req.TrunkSID); err != nil {
		WriteError(w, http.StatusInternalServerError, ErrCodeInternal, "Failed to migrate origination URLs: "+err.Error(), nil)
		return
	}

	// Get updated status
	status, err := h.deps.Twilio.GetTrunkTLSStatus(ctx, req.TrunkSID)
	if err != nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"message": "Origination URLs migrated to TLS",
		})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "Origination URLs migrated to TLS",
		"all_secure":   status.AllSecure,
		"insecure_urls": status.InsecureURLCount,
	})
}

// CreateSecureTrunkRequest represents a request to create a new secure trunk
type CreateSecureTrunkRequest struct {
	FriendlyName     string `json:"friendly_name"`
	OriginationURI   string `json:"origination_uri"`
	OriginationPriority int `json:"origination_priority"`
	OriginationWeight   int `json:"origination_weight"`
}

// CreateSecureTrunk creates a new Twilio SIP trunk with TLS enabled
func (h *TLSHandler) CreateSecureTrunk(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateSecureTrunkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.FriendlyName == "" {
		WriteValidationError(w, "friendly_name is required", nil)
		return
	}

	if h.deps.Twilio == nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeInternal, "Twilio client not available", nil)
		return
	}

	// Create trunk with secure mode enabled
	trunk, err := h.deps.Twilio.CreateSIPTrunk(ctx, req.FriendlyName, true)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrCodeInternal, "Failed to create trunk: "+err.Error(), nil)
		return
	}

	// Set origination URI if provided
	if req.OriginationURI != "" {
		priority := req.OriginationPriority
		if priority == 0 {
			priority = 10
		}
		weight := req.OriginationWeight
		if weight == 0 {
			weight = 100
		}

		// Use secure origination if URI uses sips: or port 5061
		if strings.HasPrefix(req.OriginationURI, "sips:") || strings.Contains(req.OriginationURI, ":5061") {
			err = h.deps.Twilio.SetSecureOriginationURI(ctx, trunk.SID, req.OriginationURI, priority, weight)
		} else {
			err = h.deps.Twilio.SetOriginationURI(ctx, trunk.SID, req.OriginationURI, priority, weight)
		}

		if err != nil {
			// Trunk was created but origination failed - return partial success
			WriteJSON(w, http.StatusCreated, map[string]interface{}{
				"trunk_sid":     trunk.SID,
				"friendly_name": trunk.FriendlyName,
				"secure":        trunk.Secure,
				"warning":       fmt.Sprintf("Trunk created but failed to set origination URI: %v", err),
			})
			return
		}
	}

	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"trunk_sid":     trunk.SID,
		"friendly_name": trunk.FriendlyName,
		"secure":        trunk.Secure,
		"domain_name":   trunk.DomainName,
	})
}
