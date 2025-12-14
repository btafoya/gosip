import { get, post, put } from './client'

export interface User {
  id: number
  email: string
  role: 'admin' | 'user'
  created_at: string
  last_login?: string
}

export interface LoginRequest {
  email: string
  password: string
}

export interface LoginResponse {
  user: User
  token: string
}

export interface SystemConfig {
  twilio_configured: boolean
  smtp_configured: boolean
  gotify_configured: boolean
  setup_completed: boolean
  voicemail_enabled: boolean
  recording_enabled: boolean
  transcription_enabled: boolean
}

export interface SetupRequest {
  twilio_account_sid: string
  twilio_auth_token: string
  admin_email: string
  admin_password: string
  smtp_host?: string
  smtp_port?: number
  smtp_user?: string
  smtp_password?: string
  gotify_url?: string
  gotify_token?: string
}

// Auth API
export const authApi = {
  login: (data: LoginRequest) => post<LoginResponse>('/auth/login', data),
  logout: () => post<{ message: string }>('/auth/logout'),
  getCurrentUser: () => get<User>('/me'),
  changePassword: (currentPassword: string, newPassword: string) =>
    put<{ message: string }>('/me/password', {
      current_password: currentPassword,
      new_password: newPassword
    })
}

// Setup API (public endpoints)
export const setupApi = {
  getStatus: () => get<{ setup_completed: boolean }>('/setup/status'),
  complete: (data: SetupRequest) => post<{ message: string }>('/setup/complete', data)
}

// System API (requires auth)
export const systemApi = {
  getConfig: () => get<SystemConfig>('/system/config'),
  getStatus: () => get<{
    status: string
    version: string
    uptime: string
    sip_server_status: string
    twilio_status: string
    active_calls: number
    registered_devices: number
    stats: Record<string, number>
  }>('/system/status'),
  setup: (data: SetupRequest) => post<{ message: string }>('/system/setup', data),
  updateConfig: (key: string, value: string) =>
    post<{ message: string }>('/system/config', { key, value }),
  createBackup: () => post<{ filename: string; size: number; created_at: string }>('/system/backup'),
  listBackups: () => get<Array<{ filename: string; size: number; created_at: string }>>('/system/backups')
}
