import { get, post, put, del, type PaginatedResponse } from './client'

// Types matching backend models
export interface ProvisioningProfile {
  id: number
  name: string
  vendor: string
  model?: string
  description?: string
  config_template: string
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface ProvisioningToken {
  id: number
  token: string
  device_id: number
  device_name?: string
  expires_at: string
  revoked: boolean
  max_uses: number
  use_count: number
  allowed_ip?: string
  created_at: string
  created_by: number
  last_used_at?: string
}

export interface DeviceEvent {
  id: number
  device_id: number
  event_type: 'config_fetch' | 'registration' | 'provision_complete' | 'provision_failed' | 'auth_failed' | 'error'
  event_data?: Record<string, unknown>
  ip_address?: string
  user_agent?: string
  created_at: string
}

export interface ProvisionDeviceRequest {
  device_id: number
  profile_id?: number
  vendor?: string
  model?: string
  mac_address?: string
  generate_token?: boolean
  token_expires_hours?: number
}

export interface ProvisionDeviceResponse {
  device_id: number
  config_url?: string
  token?: string
  expires_at?: string
  instructions: string
}

export interface CreateTokenRequest {
  device_id: number
  expires_in?: number // seconds
  max_uses?: number
  ip_restriction?: string
}

export interface CreateProfileRequest {
  name: string
  vendor: string
  model?: string
  description?: string
  config_template: string
  is_default?: boolean
}

export interface UpdateProfileRequest {
  name?: string
  description?: string
  config_template?: string
  is_default?: boolean
}

export interface VendorInfo {
  vendor: string
  models: string[]
  profile_count: number
}

// API methods
export const provisioningApi = {
  // Device provisioning
  provisionDevice: (data: ProvisionDeviceRequest) =>
    post<ProvisionDeviceResponse>('/provisioning/device', data),

  // Profiles - returns array directly, not paginated
  listProfiles: (params?: { vendor?: string; limit?: number; offset?: number }) =>
    get<ProvisioningProfile[]>('/provisioning/profiles', { params }),

  getProfile: (id: number) =>
    get<ProvisioningProfile>(`/provisioning/profiles/${id}`),

  createProfile: (data: CreateProfileRequest) =>
    post<ProvisioningProfile>('/provisioning/profiles', data),

  updateProfile: (id: number, data: UpdateProfileRequest) =>
    put<ProvisioningProfile>(`/provisioning/profiles/${id}`, data),

  deleteProfile: (id: number) =>
    del<{ message: string }>(`/provisioning/profiles/${id}`),

  // Vendors - returns string array directly
  listVendors: () =>
    get<string[]>('/provisioning/vendors'),

  // Tokens - returns array directly, not paginated
  listTokens: (params?: { device_id?: number; active_only?: boolean; limit?: number; offset?: number }) =>
    get<ProvisioningToken[] | null>('/provisioning/tokens', { params }),

  createToken: (data: CreateTokenRequest) =>
    post<{ token: ProvisioningToken; provisioning_url: string }>('/provisioning/tokens', data),

  revokeToken: (id: number) =>
    del<{ status: string }>(`/provisioning/tokens/${id}`),

  // QR Code for token
  getTokenQRCode: (token: string, format?: 'base64' | 'png') =>
    get<{ qr_code: string; provisioning_url: string; token: string; expires_at: string }>(
      `/provisioning/tokens/${token}/qrcode`,
      { params: { format: format || 'base64' } }
    ),

  // Events - returns array directly
  getRecentEvents: (params?: { limit?: number; event_type?: string }) =>
    get<DeviceEvent[] | null>('/provisioning/events', { params }),

  getDeviceEvents: (deviceId: number, params?: { limit?: number; offset?: number }) =>
    get<{ events: DeviceEvent[]; total: number; limit: number; offset: number }>(`/devices/${deviceId}/events`, { params })
}
