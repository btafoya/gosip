import { get, post, put, del, type PaginatedResponse } from './client'

export interface Device {
  id: number
  user_id?: number
  name: string
  username: string
  device_type: 'grandstream' | 'softphone' | 'webrtc'
  recording_enabled: boolean
  created_at: string
  online: boolean
  registered?: boolean
  extension?: string
  caller_id?: string
  user_agent?: string
  // Provisioning fields
  mac_address?: string
  vendor?: string
  model?: string
  firmware_version?: string
  provisioning_status?: 'pending' | 'provisioned' | 'failed' | 'unknown'
  last_config_fetch?: string
  last_registration?: string
  config_template?: string
}

export interface CreateDeviceRequest {
  name: string
  username: string
  password: string
  device_type: string
  recording_enabled: boolean
  user_id?: number
}

export interface UpdateDeviceRequest {
  name?: string
  password?: string
  device_type?: string
  recording_enabled?: boolean
  user_id?: number
  vendor?: string
  model?: string
}

export interface Registration {
  device_id: number
  device_name: string
  username: string
  contact: string
  user_agent: string
  expires_at: string
  last_seen: string
}

export const devicesApi = {
  list: (params?: { limit?: number; offset?: number }) =>
    get<PaginatedResponse<Device>>('/devices', { params }),

  get: (id: number) => get<Device>(`/devices/${id}`),

  create: (data: CreateDeviceRequest) => post<Device>('/devices', data),

  update: (id: number, data: UpdateDeviceRequest) => put<Device>(`/devices/${id}`, data),

  delete: (id: number) => del<{ message: string }>(`/devices/${id}`),

  getRegistrations: () => get<Registration[]>('/devices/registrations')
}
