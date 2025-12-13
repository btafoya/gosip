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
func (c *Client) CreateSIPTrunk(ctx context.Context, friendlyName string) (*SIPTrunk, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &trunking.CreateTrunkParams{}
	params.SetFriendlyName(friendlyName)
	params.SetSecure(true)

	resp, err := client.TrunkingV1.CreateTrunk(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create SIP trunk: %w", err)
	}

	trunk := &SIPTrunk{
		FriendlyName: friendlyName,
		Secure:       true,
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
