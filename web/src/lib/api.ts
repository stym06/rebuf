const BASE = '/api/v1';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = localStorage.getItem('token');
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };

  const res = await fetch(`${BASE}${path}`, { ...options, headers });

  if (res.status === 204) return undefined as T;

  const data = await res.json();
  if (!res.ok) throw new Error(data.message || 'Request failed');
  return data as T;
}

export const api = {
  // Auth
  signup: (body: { email: string; password: string; name: string; org_name: string }) =>
    request<{ token: string; user: any; org: any }>('/auth/signup', {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  login: (body: { email: string; password: string }) =>
    request<{ token: string; user: any; org: any }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  // Applications
  listApps: () => request<{ data: any[] }>('/app/'),
  createApp: (body: { name: string; uid?: string }) =>
    request<any>('/app/', { method: 'POST', body: JSON.stringify(body) }),
  getApp: (id: string) => request<any>(`/app/${id}`),
  deleteApp: (id: string) => request<void>(`/app/${id}`, { method: 'DELETE' }),

  // Endpoints
  listEndpoints: (appId: string) =>
    request<{ data: any[] }>(`/app/${appId}/endpoint/`),
  createEndpoint: (appId: string, body: { url: string; description?: string; filter_types?: string[] }) =>
    request<any>(`/app/${appId}/endpoint/`, { method: 'POST', body: JSON.stringify(body) }),
  deleteEndpoint: (appId: string, epId: string) =>
    request<void>(`/app/${appId}/endpoint/${epId}`, { method: 'DELETE' }),

  // Messages
  listMessages: (appId: string) =>
    request<{ data: any[] }>(`/app/${appId}/msg/`),
  createMessage: (appId: string, body: { event_type: string; payload: any; event_id?: string }) =>
    request<any>(`/app/${appId}/msg/`, { method: 'POST', body: JSON.stringify(body) }),
  listAttempts: (appId: string, msgId: string) =>
    request<{ data: any[] }>(`/app/${appId}/msg/${msgId}/attempt`),

  // Event Types
  listEventTypes: () => request<{ data: any[] }>('/event-type/'),
  createEventType: (body: { name: string; description?: string }) =>
    request<any>('/event-type/', { method: 'POST', body: JSON.stringify(body) }),

  // API Keys
  listAPIKeys: () => request<{ data: any[] }>('/api-keys/'),
  createAPIKey: (body: { name: string }) =>
    request<{ api_key: any; key: string }>('/api-keys/', { method: 'POST', body: JSON.stringify(body) }),
  deleteAPIKey: (id: string) => request<void>(`/api-keys/${id}`, { method: 'DELETE' }),
};
