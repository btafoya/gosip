package twilio

import (
	"context"
	"fmt"

	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
	trunking "github.com/twilio/twilio-go/rest/trunking/v1"
)

// SIPTrunk represents a Twilio SIP trunk configuration
type SIPTrunk struct {
	SID          string
	FriendlyName string
	DomainName   string
	Secure       bool
	// TLS-specific fields
	TransferMode string // "disable-all", "enable-all", "sip-only"
	CnamLookupEnabled bool
}

// OriginationURL represents a Twilio origination URL (where calls route TO from Twilio)
type OriginationURL struct {
	SID          string
	SipURL       string
	FriendlyName string
	Priority     int
	Weight       int
	Enabled      bool
}

// SIPDomain represents a Twilio SIP domain
type SIPDomain struct {
	SID                    string
	DomainName             string
	FriendlyName           string
	VoiceURL               string
	VoiceFallbackURL       string
	VoiceStatusCallbackURL string
}

// CreateSIPTrunk creates a new SIP trunk
func (c *Client) CreateSIPTrunk(ctx context.Context, friendlyName string, secure bool) (*SIPTrunk, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &trunking.CreateTrunkParams{}
	params.SetFriendlyName(friendlyName)
	params.SetSecure(secure)

	resp, err := client.TrunkingV1.CreateTrunk(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create SIP trunk: %w", err)
	}

	trunk := &SIPTrunk{
		FriendlyName: friendlyName,
		Secure:       secure,
	}

	if resp.Sid != nil {
		trunk.SID = *resp.Sid
	}
	if resp.DomainName != nil {
		trunk.DomainName = *resp.DomainName
	}

	return trunk, nil
}

// GetSIPTrunk retrieves a SIP trunk by SID
func (c *Client) GetSIPTrunk(ctx context.Context, trunkSID string) (*SIPTrunk, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	resp, err := client.TrunkingV1.FetchTrunk(trunkSID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SIP trunk: %w", err)
	}

	trunk := &SIPTrunk{
		SID: trunkSID,
	}

	if resp.FriendlyName != nil {
		trunk.FriendlyName = *resp.FriendlyName
	}
	if resp.DomainName != nil {
		trunk.DomainName = *resp.DomainName
	}
	if resp.Secure != nil {
		trunk.Secure = *resp.Secure
	}

	return trunk, nil
}

// ListSIPTrunks lists all SIP trunks
func (c *Client) ListSIPTrunks(ctx context.Context) ([]*SIPTrunk, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	resp, err := client.TrunkingV1.ListTrunk(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list SIP trunks: %w", err)
	}

	var trunks []*SIPTrunk
	for _, t := range resp {
		trunk := &SIPTrunk{}
		if t.Sid != nil {
			trunk.SID = *t.Sid
		}
		if t.FriendlyName != nil {
			trunk.FriendlyName = *t.FriendlyName
		}
		if t.DomainName != nil {
			trunk.DomainName = *t.DomainName
		}
		if t.Secure != nil {
			trunk.Secure = *t.Secure
		}
		trunks = append(trunks, trunk)
	}

	return trunks, nil
}

// DeleteSIPTrunk deletes a SIP trunk
func (c *Client) DeleteSIPTrunk(ctx context.Context, trunkSID string) error {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	err := client.TrunkingV1.DeleteTrunk(trunkSID)
	if err != nil {
		return fmt.Errorf("failed to delete SIP trunk: %w", err)
	}

	return nil
}

// AssignPhoneNumberToTrunk assigns a phone number to a SIP trunk
func (c *Client) AssignPhoneNumberToTrunk(ctx context.Context, trunkSID, phoneNumberSID string) error {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &trunking.CreatePhoneNumberParams{}
	params.SetPhoneNumberSid(phoneNumberSID)

	_, err := client.TrunkingV1.CreatePhoneNumber(trunkSID, params)
	if err != nil {
		return fmt.Errorf("failed to assign phone number to trunk: %w", err)
	}

	return nil
}

// CreateSIPDomain creates a new SIP domain
func (c *Client) CreateSIPDomain(ctx context.Context, domainName, friendlyName, voiceURL string) (*SIPDomain, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &twilioApi.CreateSipDomainParams{}
	params.SetDomainName(domainName)
	params.SetFriendlyName(friendlyName)
	if voiceURL != "" {
		params.SetVoiceUrl(voiceURL)
	}

	resp, err := client.Api.CreateSipDomain(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create SIP domain: %w", err)
	}

	domain := &SIPDomain{
		DomainName:   domainName,
		FriendlyName: friendlyName,
		VoiceURL:     voiceURL,
	}

	if resp.Sid != nil {
		domain.SID = *resp.Sid
	}

	return domain, nil
}

// GetSIPDomain retrieves a SIP domain by SID
func (c *Client) GetSIPDomain(ctx context.Context, domainSID string) (*SIPDomain, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	resp, err := client.Api.FetchSipDomain(domainSID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SIP domain: %w", err)
	}

	domain := &SIPDomain{
		SID: domainSID,
	}

	if resp.DomainName != nil {
		domain.DomainName = *resp.DomainName
	}
	if resp.FriendlyName != nil {
		domain.FriendlyName = *resp.FriendlyName
	}
	if resp.VoiceUrl != nil {
		domain.VoiceURL = *resp.VoiceUrl
	}
	if resp.VoiceFallbackUrl != nil {
		domain.VoiceFallbackURL = *resp.VoiceFallbackUrl
	}
	if resp.VoiceStatusCallbackUrl != nil {
		domain.VoiceStatusCallbackURL = *resp.VoiceStatusCallbackUrl
	}

	return domain, nil
}

// UpdateSIPDomain updates a SIP domain
func (c *Client) UpdateSIPDomain(ctx context.Context, domainSID, voiceURL, voiceFallbackURL, voiceStatusCallbackURL string) error {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &twilioApi.UpdateSipDomainParams{}
	if voiceURL != "" {
		params.SetVoiceUrl(voiceURL)
	}
	if voiceFallbackURL != "" {
		params.SetVoiceFallbackUrl(voiceFallbackURL)
	}
	if voiceStatusCallbackURL != "" {
		params.SetVoiceStatusCallbackUrl(voiceStatusCallbackURL)
	}

	_, err := client.Api.UpdateSipDomain(domainSID, params)
	if err != nil {
		return fmt.Errorf("failed to update SIP domain: %w", err)
	}

	return nil
}

// DeleteSIPDomain deletes a SIP domain
func (c *Client) DeleteSIPDomain(ctx context.Context, domainSID string) error {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	err := client.Api.DeleteSipDomain(domainSID, nil)
	if err != nil {
		return fmt.Errorf("failed to delete SIP domain: %w", err)
	}

	return nil
}

// CreateCredentialList creates a new credential list for SIP authentication
func (c *Client) CreateCredentialList(ctx context.Context, friendlyName string) (string, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return "", fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &twilioApi.CreateSipCredentialListParams{}
	params.SetFriendlyName(friendlyName)

	resp, err := client.Api.CreateSipCredentialList(params)
	if err != nil {
		return "", fmt.Errorf("failed to create credential list: %w", err)
	}

	if resp.Sid == nil {
		return "", fmt.Errorf("no SID returned")
	}

	return *resp.Sid, nil
}

// AddCredential adds a credential to a credential list
func (c *Client) AddCredential(ctx context.Context, credentialListSID, username, password string) error {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &twilioApi.CreateSipCredentialParams{}
	params.SetUsername(username)
	params.SetPassword(password)

	_, err := client.Api.CreateSipCredential(credentialListSID, params)
	if err != nil {
		return fmt.Errorf("failed to add credential: %w", err)
	}

	return nil
}

// MapCredentialListToDomain maps a credential list to a SIP domain
func (c *Client) MapCredentialListToDomain(ctx context.Context, domainSID, credentialListSID string) error {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &twilioApi.CreateSipAuthCallsCredentialListMappingParams{}
	params.SetCredentialListSid(credentialListSID)

	_, err := client.Api.CreateSipAuthCallsCredentialListMapping(domainSID, params)
	if err != nil {
		return fmt.Errorf("failed to map credential list to domain: %w", err)
	}

	return nil
}

// SetOriginationURI sets the origination URI for a SIP trunk
func (c *Client) SetOriginationURI(ctx context.Context, trunkSID, sipURI string, priority, weight int) error {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &trunking.CreateOriginationUrlParams{}
	params.SetSipUrl(sipURI)
	params.SetFriendlyName("Primary")
	params.SetPriority(priority)
	params.SetWeight(weight)
	params.SetEnabled(true)

	_, err := client.TrunkingV1.CreateOriginationUrl(trunkSID, params)
	if err != nil {
		return fmt.Errorf("failed to set origination URI: %w", err)
	}

	return nil
}

// UpdateSIPTrunk updates a SIP trunk's configuration
func (c *Client) UpdateSIPTrunk(ctx context.Context, trunkSID string, secure bool, friendlyName string) error {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &trunking.UpdateTrunkParams{}
	params.SetSecure(secure)
	if friendlyName != "" {
		params.SetFriendlyName(friendlyName)
	}

	_, err := client.TrunkingV1.UpdateTrunk(trunkSID, params)
	if err != nil {
		return fmt.Errorf("failed to update SIP trunk: %w", err)
	}

	return nil
}

// EnableTLSForTrunk enables TLS (secure mode) for a SIP trunk
func (c *Client) EnableTLSForTrunk(ctx context.Context, trunkSID string) error {
	return c.UpdateSIPTrunk(ctx, trunkSID, true, "")
}

// DisableTLSForTrunk disables TLS for a SIP trunk (not recommended)
func (c *Client) DisableTLSForTrunk(ctx context.Context, trunkSID string) error {
	return c.UpdateSIPTrunk(ctx, trunkSID, false, "")
}

// SetSecureOriginationURI sets a secure (TLS) origination URI for a SIP trunk
// The sipURI should use sips: scheme for TLS, e.g., "sips:your-server.com:5061"
func (c *Client) SetSecureOriginationURI(ctx context.Context, trunkSID, sipURI string, priority, weight int) error {
	// Validate that the URI uses TLS (sips: scheme or port 5061)
	if !isSecureSIPURI(sipURI) {
		return fmt.Errorf("origination URI must use sips: scheme or port 5061 for TLS: %s", sipURI)
	}
	return c.SetOriginationURI(ctx, trunkSID, sipURI, priority, weight)
}

// isSecureSIPURI validates that a SIP URI is using TLS
func isSecureSIPURI(uri string) bool {
	// Check for sips: scheme (TLS)
	if len(uri) >= 5 && uri[:5] == "sips:" {
		return true
	}
	// Check for port 5061 (standard SIPS port)
	if contains(uri, ":5061") {
		return true
	}
	return false
}

// contains is a simple string contains helper
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ListOriginationURLs lists all origination URLs for a trunk
func (c *Client) ListOriginationURLs(ctx context.Context, trunkSID string) ([]*OriginationURL, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	resp, err := client.TrunkingV1.ListOriginationUrl(trunkSID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list origination URLs: %w", err)
	}

	var urls []*OriginationURL
	for _, o := range resp {
		url := &OriginationURL{}
		if o.Sid != nil {
			url.SID = *o.Sid
		}
		if o.SipUrl != nil {
			url.SipURL = *o.SipUrl
		}
		if o.FriendlyName != nil {
			url.FriendlyName = *o.FriendlyName
		}
		if o.Priority != nil {
			url.Priority = *o.Priority
		}
		if o.Weight != nil {
			url.Weight = *o.Weight
		}
		if o.Enabled != nil {
			url.Enabled = *o.Enabled
		}
		urls = append(urls, url)
	}

	return urls, nil
}

// UpdateOriginationURL updates an existing origination URL
func (c *Client) UpdateOriginationURL(ctx context.Context, trunkSID, originationURLSID, sipURI string, priority, weight int, enabled bool) error {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &trunking.UpdateOriginationUrlParams{}
	if sipURI != "" {
		params.SetSipUrl(sipURI)
	}
	params.SetPriority(priority)
	params.SetWeight(weight)
	params.SetEnabled(enabled)

	_, err := client.TrunkingV1.UpdateOriginationUrl(trunkSID, originationURLSID, params)
	if err != nil {
		return fmt.Errorf("failed to update origination URL: %w", err)
	}

	return nil
}

// DeleteOriginationURL deletes an origination URL from a trunk
func (c *Client) DeleteOriginationURL(ctx context.Context, trunkSID, originationURLSID string) error {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	err := client.TrunkingV1.DeleteOriginationUrl(trunkSID, originationURLSID)
	if err != nil {
		return fmt.Errorf("failed to delete origination URL: %w", err)
	}

	return nil
}

// MigrateToSecureOrigination migrates all origination URLs to TLS
// This updates existing origination URLs to use sips: scheme
func (c *Client) MigrateToSecureOrigination(ctx context.Context, trunkSID string) error {
	urls, err := c.ListOriginationURLs(ctx, trunkSID)
	if err != nil {
		return err
	}

	for _, url := range urls {
		if !isSecureSIPURI(url.SipURL) {
			// Convert sip: to sips: and port 5060 to 5061
			secureURL := convertToSecureURI(url.SipURL)
			err := c.UpdateOriginationURL(ctx, trunkSID, url.SID, secureURL, url.Priority, url.Weight, url.Enabled)
			if err != nil {
				return fmt.Errorf("failed to migrate origination URL %s: %w", url.SID, err)
			}
		}
	}

	return nil
}

// convertToSecureURI converts a SIP URI to its secure equivalent
func convertToSecureURI(uri string) string {
	// Replace sip: with sips:
	if len(uri) >= 4 && uri[:4] == "sip:" {
		uri = "sips:" + uri[4:]
	}
	// Replace :5060 with :5061
	for i := 0; i <= len(uri)-5; i++ {
		if uri[i:i+5] == ":5060" {
			uri = uri[:i] + ":5061" + uri[i+5:]
			break
		}
	}
	return uri
}

// TrunkTLSStatus represents the TLS status of a SIP trunk and its connections
type TrunkTLSStatus struct {
	TrunkSID         string
	FriendlyName     string
	SecureMode       bool
	OriginationURLs  []*OriginationURL
	AllSecure        bool
	InsecureURLCount int
}

// GetTrunkTLSStatus returns the TLS status for a trunk
func (c *Client) GetTrunkTLSStatus(ctx context.Context, trunkSID string) (*TrunkTLSStatus, error) {
	trunk, err := c.GetSIPTrunk(ctx, trunkSID)
	if err != nil {
		return nil, err
	}

	urls, err := c.ListOriginationURLs(ctx, trunkSID)
	if err != nil {
		return nil, err
	}

	status := &TrunkTLSStatus{
		TrunkSID:        trunk.SID,
		FriendlyName:    trunk.FriendlyName,
		SecureMode:      trunk.Secure,
		OriginationURLs: urls,
		AllSecure:       trunk.Secure,
	}

	// Check if all origination URLs are secure
	for _, url := range urls {
		if !isSecureSIPURI(url.SipURL) {
			status.InsecureURLCount++
			status.AllSecure = false
		}
	}

	return status, nil
}

// EnsureTrunkFullySecure ensures a trunk is fully configured for TLS
// This enables secure mode AND migrates all origination URLs to TLS
func (c *Client) EnsureTrunkFullySecure(ctx context.Context, trunkSID string) error {
	// First, enable TLS on the trunk
	if err := c.EnableTLSForTrunk(ctx, trunkSID); err != nil {
		return fmt.Errorf("failed to enable TLS on trunk: %w", err)
	}

	// Then, migrate all origination URLs to TLS
	if err := c.MigrateToSecureOrigination(ctx, trunkSID); err != nil {
		return fmt.Errorf("failed to migrate origination URLs: %w", err)
	}

	return nil
}
