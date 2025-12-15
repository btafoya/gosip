import apiClient from './client'

export interface ActiveCall {
  call_id: string
  direction: 'inbound' | 'outbound'
  state: 'ringing' | 'active' | 'held' | 'holding' | 'transferring' | 'terminated'
  from_number: string
  to_number: string
  duration: number
  device_id?: number
  local_uri: string
  remote_uri: string
  transfer_target?: string
  consult_call_id?: string
  transferred_from?: string
}

export interface MOHStatus {
  enabled: boolean
  audio_path: string
  active_count: number
}

export interface HoldRequest {
  hold: boolean
}

export interface TransferRequest {
  type: 'blind' | 'attended'
  target?: string
  consult_id?: string
}

export interface MOHUpdateRequest {
  enabled?: boolean
  audio_path?: string
}

export const callsApi = {
  // List all active calls
  async listActiveCalls(): Promise<{ data: ActiveCall[]; count: number }> {
    const response = await apiClient.get<{ data: ActiveCall[]; count: number }>('/calls')
    return response.data
  },

  // Get a specific call
  async getCall(callId: string): Promise<{ data: ActiveCall }> {
    const response = await apiClient.get<{ data: ActiveCall }>(`/calls/${encodeURIComponent(callId)}`)
    return response.data
  },

  // Hold or resume a call
  async holdCall(callId: string, hold: boolean): Promise<{ success: boolean; state: string }> {
    const response = await apiClient.post<{ success: boolean; state: string }>(
      `/calls/${encodeURIComponent(callId)}/hold`,
      { hold } as HoldRequest
    )
    return response.data
  },

  // Transfer a call
  async transferCall(
    callId: string,
    type: 'blind' | 'attended',
    target?: string,
    consultId?: string
  ): Promise<{ success: boolean; state: string }> {
    const response = await apiClient.post<{ success: boolean; state: string }>(
      `/calls/${encodeURIComponent(callId)}/transfer`,
      { type, target, consult_id: consultId } as TransferRequest
    )
    return response.data
  },

  // Cancel a transfer
  async cancelTransfer(callId: string): Promise<{ success: boolean; state: string }> {
    const response = await apiClient.delete<{ success: boolean; state: string }>(
      `/calls/${encodeURIComponent(callId)}/transfer`
    )
    return response.data
  },

  // Hang up a call
  async hangupCall(callId: string): Promise<{ success: boolean }> {
    const response = await apiClient.delete<{ success: boolean }>(
      `/calls/${encodeURIComponent(callId)}`
    )
    return response.data
  },

  // Get MOH status
  async getMOHStatus(): Promise<{ data: MOHStatus }> {
    const response = await apiClient.get<{ data: MOHStatus }>('/calls/moh')
    return response.data
  },

  // Update MOH settings
  async updateMOH(settings: MOHUpdateRequest): Promise<{ success: boolean; data: MOHStatus }> {
    const response = await apiClient.put<{ success: boolean; data: MOHStatus }>(
      '/calls/moh',
      settings
    )
    return response.data
  }
}

export default callsApi
