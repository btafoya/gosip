import { get, post, put, del, type PaginatedResponse } from './client'

export interface Trunk {
  id: number
  twilio_sid: string
  friendly_name: string
  domain_name: string
  secure: boolean
  transfer_mode: string
  cnam_lookup_enabled: boolean
  created_at: string
  updated_at: string
}

export interface CreateTrunkRequest {
  friendly_name: string
  secure: boolean
  transfer_mode?: string
  cnam_lookup_enabled?: boolean
}

export interface UpdateTrunkRequest {
  friendly_name?: string
  secure?: boolean
  transfer_mode?: string
  cnam_lookup_enabled?: boolean
}

export interface AssignDIDRequest {
  did_id: number
}

export interface SyncSummary {
  message: string
  created: number
  updated: number
  total: number
  trunks: Trunk[]
}

export const trunksApi = {
  list: () => get<{ data: Trunk[] }>('/trunks'),

  get: (id: number) => get<Trunk>(`/trunks/${id}`),

  create: (data: CreateTrunkRequest) => post<Trunk>('/trunks', data),

  update: (id: number, data: UpdateTrunkRequest) => put<Trunk>(`/trunks/${id}`, data),

  delete: (id: number) => del<{ message: string }>(`/trunks/${id}`),

  sync: () => post<SyncSummary>('/trunks/sync'),

  assignDID: (id: number, data: AssignDIDRequest) =>
    post<{ message: string }>(`/trunks/${id}/assign-did`, data),

  unassignDID: (id: number, data: AssignDIDRequest) =>
    post<{ message: string }>(`/trunks/${id}/unassign-did`, data),
}
