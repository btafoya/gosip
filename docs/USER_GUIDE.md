# GoSIP User Guide

Welcome to GoSIP! This guide covers everything you need to know to use the system as an end user.

---

## Table of Contents

1. [Getting Started](#getting-started)
2. [Dashboard](#dashboard)
3. [Voicemail](#voicemail)
4. [Messages (SMS/MMS)](#messages)
5. [Call History](#call-history)
6. [Your Devices](#your-devices)
7. [Settings](#settings)
8. [Troubleshooting](#troubleshooting)

---

## Getting Started

### Logging In

1. Open your web browser and navigate to your GoSIP server (e.g., `http://your-server:8080`)
2. Enter your **email address** and **password**
3. Click **Login**

Your administrator will provide your login credentials. If you've forgotten your password, contact your system administrator.

### First-Time Setup

When you log in for the first time:
1. You'll be taken to the **Dashboard**
2. Review your assigned phone number(s) (DIDs)
3. Check that your SIP device is registered (green status)

---

## Dashboard

The Dashboard is your home screen, showing an overview of your communications.

### What You'll See

| Section | Description |
|---------|-------------|
| **Unread Voicemails** | Count and preview of new voicemails |
| **Unread Messages** | Count and preview of new SMS/MMS messages |
| **Recent Calls** | Last 5-10 incoming/outgoing calls |
| **Device Status** | Registration status of your SIP phone(s) |
| **Your Numbers** | Phone numbers (DIDs) assigned to you |

### Quick Actions

From the Dashboard, you can:
- Click on a voicemail to listen
- Click on a message to view the conversation
- Click on a call to see details
- Access any section from the navigation menu

---

## Voicemail

### Checking Voicemails

1. Click **Voicemails** in the navigation menu
2. New (unread) voicemails appear at the top with a badge
3. Click on any voicemail to expand it

### Voicemail Details

Each voicemail shows:
- **Caller Number** - Who left the message
- **Date/Time** - When it was received
- **Duration** - Length of the message
- **Transcript** - Text version of the audio (if available)
- **Audio Player** - Listen to the original recording

### Listening to Voicemails

1. Click the **Play** button on the audio player
2. Use the progress bar to skip forward/backward
3. Adjust volume as needed

### Managing Voicemails

| Action | How To |
|--------|--------|
| **Mark as Read** | Click the voicemail, or click the checkmark icon |
| **Delete** | Click the trash icon (cannot be undone) |
| **Call Back** | Click the phone icon next to the caller's number |

### Voicemail Indicator (MWI)

If your desk phone supports Message Waiting Indicator (MWI):
- The voicemail light turns **ON** when you have new messages
- The light turns **OFF** when all voicemails are read or deleted

---

## Messages

GoSIP supports SMS and MMS messaging through your assigned phone numbers.

### Viewing Messages

1. Click **Messages** in the navigation menu
2. Messages are grouped by **conversation** (contact number)
3. Click a conversation to see the full thread

### Conversation View

The conversation view shows:
- All messages with a specific contact
- **Incoming messages** appear on the left (gray)
- **Outgoing messages** appear on the right (blue)
- Date/time stamps between message groups

### Sending a New Message

1. Click **New Message** or the compose button
2. Enter the recipient's phone number (with country code, e.g., +1...)
3. Select which of your numbers to send from
4. Type your message
5. Click **Send**

### Sending Pictures (MMS)

1. In the compose area, click the **attachment** icon
2. Select an image file from your device
3. Add optional text
4. Click **Send**

**Note**: MMS is supported for images. Maximum file size depends on your carrier.

### Message Status

| Status | Meaning |
|--------|---------|
| **Sending** | Message is being sent to carrier |
| **Sent** | Message accepted by carrier |
| **Delivered** | Message delivered to recipient's phone |
| **Failed** | Message could not be sent (check number) |

### Managing Messages

| Action | How To |
|--------|--------|
| **Mark as Read** | Open the conversation |
| **Delete Message** | Click the trash icon on individual message |
| **Delete Conversation** | Click menu → Delete Conversation |

---

## Call History

View all your incoming and outgoing calls.

### Accessing Call History

1. Click **Calls** in the navigation menu
2. Calls are listed newest first

### Call Record Details

Each call shows:
- **Direction** - Incoming (↓) or Outgoing (↑)
- **Contact Number** - Who you called or who called you
- **Your Number** - Which DID was used
- **Date/Time** - When the call occurred
- **Duration** - How long the call lasted
- **Status** - Answered, Missed, Voicemail, etc.

### Call Status Types

| Status | Description |
|--------|-------------|
| **Answered** | Call was connected and answered |
| **Missed** | Incoming call that wasn't answered |
| **Voicemail** | Call went to voicemail |
| **Blocked** | Call was blocked by system rules |
| **Busy** | Called party was busy |
| **Failed** | Call could not be connected |

### Filtering Calls

Use the filters to find specific calls:
- **All Calls** - Show everything
- **Incoming** - Show only received calls
- **Outgoing** - Show only calls you made
- **Missed** - Show only unanswered calls

### Call Actions

| Action | How To |
|--------|--------|
| **Call Back** | Click the phone icon |
| **Send Message** | Click the message icon |
| **View Recording** | Click play button (if recording enabled) |
| **Block Number** | Click menu → Block this caller |

---

## Your Devices

Manage your SIP phones and softphones.

### Device Status

The Devices page shows all phones registered to your account:

| Status | Meaning |
|--------|---------|
| **Online** (Green) | Device is registered and ready |
| **Offline** (Gray) | Device is not connected |
| **Ringing** | Device is currently ringing |
| **On Call** | Device is on an active call |

### Device Information

For each device, you can see:
- **Device Name** - Friendly name (e.g., "Office Phone")
- **Device Type** - Grandstream, Softphone, etc.
- **IP Address** - Current network address
- **Last Seen** - When device last communicated

### Setting Up a New Device

If your administrator has given you permission to add devices:

1. Click **Add Device**
2. Enter a friendly name
3. Choose device type
4. Note the credentials (username/password)
5. Configure your phone with these settings

**SIP Settings for Your Phone:**
| Setting | Value |
|---------|-------|
| SIP Server | Your GoSIP server address |
| SIP Port | 5060 (or 5061 for TLS) |
| Username | Provided by system |
| Password | Provided by system |
| Transport | UDP (or TCP/TLS) |

### Device Provisioning (Auto-Setup)

If your phone supports auto-provisioning:

1. Ask your administrator for a **provisioning URL**
2. Enter it in your phone's provisioning settings
3. Reboot your phone
4. Settings will be configured automatically

Some devices support **QR Code** provisioning - scan the code shown in the web interface.

---

## Settings

Personalize your GoSIP experience.

### Accessing Settings

Click **Settings** in the navigation menu or click your profile icon.

### Available Settings

#### Profile
- **Email** - Your login email (contact admin to change)
- **Change Password** - Update your password

#### Notifications
- **Email Notifications** - Get emails for voicemails/messages
- **Voicemail Email** - Receive voicemails as email attachments

#### Preferences
- **Time Zone** - Set your local time zone
- **Date Format** - Choose date display format

### Changing Your Password

1. Go to **Settings** → **Profile**
2. Click **Change Password**
3. Enter your current password
4. Enter your new password (twice)
5. Click **Save**

**Password Requirements:**
- Minimum 8 characters
- At least one uppercase letter
- At least one number

---

## Troubleshooting

### Phone Not Registering

**Problem**: Your SIP phone shows "Offline" or won't connect.

**Solutions**:
1. Check your network connection
2. Verify SIP server address is correct
3. Confirm username and password
4. Try restarting your phone
5. Check if firewall is blocking SIP (port 5060)

### Can't Hear Audio on Calls

**Problem**: Call connects but no audio.

**Solutions**:
1. Check phone volume settings
2. Ensure RTP ports aren't blocked (10000-20000)
3. If behind NAT, ensure STUN is configured
4. Try using TCP transport instead of UDP

### Voicemail Light Won't Turn Off

**Problem**: MWI light stays on after reading voicemails.

**Solutions**:
1. Ensure all voicemails are marked as read
2. Refresh the voicemail page
3. Click "Sync MWI" if available
4. Wait a few minutes for the indicator to update

### Messages Not Sending

**Problem**: SMS messages fail to send.

**Solutions**:
1. Verify recipient number format (+1XXXXXXXXXX)
2. Check you have a DID with SMS enabled
3. Ensure your account has SMS credits
4. Contact administrator if problem persists

### Calls Going Straight to Voicemail

**Problem**: Incoming calls don't ring your phone.

**Possible Causes**:
1. **Phone offline** - Check device status
2. **DND enabled** - Check Do Not Disturb setting
3. **After hours** - Routing rules may send calls to voicemail
4. **Phone unplugged/disconnected**

### Forgot Password

Contact your system administrator to reset your password. For security, passwords cannot be recovered, only reset.

---

## Getting Help

If you need assistance:

1. **Check this guide** for common solutions
2. **Contact your administrator** for account issues
3. **Report technical issues** through your IT helpdesk

---

## Quick Reference

### Keyboard Shortcuts (Web Interface)

| Shortcut | Action |
|----------|--------|
| `N` | New message |
| `R` | Refresh current page |
| `?` | Show keyboard shortcuts |

### Phone Codes (If Supported)

| Code | Action |
|------|--------|
| `*97` | Check voicemail |
| `*72` | Enable call forwarding |
| `*73` | Disable call forwarding |
| `*78` | Enable Do Not Disturb |
| `*79` | Disable Do Not Disturb |

*Note: Feature codes depend on your system configuration. Ask your administrator for your specific codes.*

---

**Version**: 1.0
**Last Updated**: December 2025
