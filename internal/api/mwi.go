package api

import (
	"context"
	"fmt"
	"log/slog"
)

// MWINotifier handles MWI notification triggers
type MWINotifier struct {
	deps *Dependencies
}

// NewMWINotifier creates a new MWI notifier
func NewMWINotifier(deps *Dependencies) *MWINotifier {
	return &MWINotifier{deps: deps}
}

// UpdateMWIForDID updates MWI state for all devices associated with a DID
// This should be called when voicemails are created, read, or deleted
func (n *MWINotifier) UpdateMWIForDID(ctx context.Context, didID int64) error {
	if n.deps.SIP == nil {
		slog.Debug("SIP server not available, skipping MWI notification")
		return nil
	}

	mwiMgr := n.deps.SIP.GetMWIManager()
	if mwiMgr == nil {
		slog.Debug("MWI manager not available, skipping MWI notification")
		return nil
	}

	// Get voicemail counts for this DID
	newCount, err := n.deps.DB.Voicemails.CountUnread(ctx, &didID)
	if err != nil {
		return fmt.Errorf("failed to count unread voicemails: %w", err)
	}

	totalCount, err := n.deps.DB.Voicemails.CountByUser(ctx, didID)
	if err != nil {
		return fmt.Errorf("failed to count total voicemails: %w", err)
	}

	oldCount := totalCount - newCount
	if oldCount < 0 {
		oldCount = 0
	}

	// Get devices associated with this DID to find their AORs
	devices, err := n.deps.DB.Devices.ListByUser(ctx, didID)
	if err != nil {
		return fmt.Errorf("failed to list devices for DID: %w", err)
	}

	// Get DID info for domain construction
	did, err := n.deps.DB.DIDs.GetByID(ctx, didID)
	if err != nil {
		slog.Warn("Could not find DID for MWI update",
			slog.Int64("did_id", didID),
			slog.String("error", err.Error()),
		)
		// Continue with default domain
	}

	// Update MWI state for each device
	for _, device := range devices {
		// Construct AOR in standard SIP format
		// The domain should match what devices use when subscribing
		domain := "gosip" // Default domain
		_ = did           // DID info available for future enhancements

		aor := fmt.Sprintf("sip:%s@%s", device.Username, domain)

		slog.Debug("Updating MWI state",
			slog.String("aor", aor),
			slog.Int("new_messages", newCount),
			slog.Int("old_messages", oldCount),
		)

		if err := mwiMgr.UpdateState(ctx, aor, newCount, oldCount); err != nil {
			slog.Error("Failed to update MWI state",
				slog.String("aor", aor),
				slog.String("error", err.Error()),
			)
			// Continue with other devices even if one fails
		}
	}

	return nil
}

// UpdateMWIForVoicemail updates MWI for a specific voicemail's DID
func (n *MWINotifier) UpdateMWIForVoicemail(ctx context.Context, voicemailID int64) error {
	// Get the voicemail to find its DID
	voicemail, err := n.deps.DB.Voicemails.GetByID(ctx, voicemailID)
	if err != nil {
		return fmt.Errorf("failed to get voicemail: %w", err)
	}

	if voicemail.UserID == nil {
		slog.Debug("Voicemail has no associated DID, skipping MWI update")
		return nil
	}

	return n.UpdateMWIForDID(ctx, *voicemail.UserID)
}
