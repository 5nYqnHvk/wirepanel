import axios, { AxiosInstance } from 'axios'

const TOKEN_KEY = 'wp_token'
const ROLE_KEY = 'wp_role'
const USER_KEY = 'wp_user'

export type Role = 'viewer' | 'operator' | 'admin'

export function getToken(): string | null { return localStorage.getItem(TOKEN_KEY) }
export function setToken(t: string | null) { t ? localStorage.setItem(TOKEN_KEY, t) : localStorage.removeItem(TOKEN_KEY) }
export function getRole(): Role | null { return localStorage.getItem(ROLE_KEY) as Role | null }
export function setRole(r: Role | null) { r ? localStorage.setItem(ROLE_KEY, r) : localStorage.removeItem(ROLE_KEY) }
export function getUser(): string | null { return localStorage.getItem(USER_KEY) }
export function setUser(u: string | null) { u ? localStorage.setItem(USER_KEY, u) : localStorage.removeItem(USER_KEY) }

export const http: AxiosInstance = axios.create({ baseURL: '/api' })

http.interceptors.request.use((cfg) => {
  const t = getToken()
  if (t) cfg.headers.Authorization = `Bearer ${t}`
  return cfg
})

http.interceptors.response.use(
  (r) => r,
  (err) => {
    if (err?.response?.status === 401) {
      setToken(null); setRole(null); setUser(null)
      if (location.pathname !== '/login') location.href = '/login'
    }
    return Promise.reject(err)
  }
)

export interface Agent {
  id: string; hostname: string; os: string; arch: string; version: string
  connected_at: string; last_seen: string; online: boolean
}

export interface SystemInfo {
  hostname: string; os: string; arch: string; kernel?: string; distro?: string
  uptime_sec: number; cpu_count: number
  load_avg_1: number; load_avg_5: number; load_avg_15: number
  mem_total_kb: number; mem_free_kb: number; mem_avail_kb: number
}

export interface Service { name: string; description?: string; load?: string; active?: string; sub?: string }

export interface FSEntry {
  name: string; path: string; size: number; mode: string; is_dir: boolean
  mod_time: string; owner?: string
}

export interface AuditEntry {
  id: string; timestamp: string; user: string; role: string; ip?: string
  agent_id?: string; action: string; target?: string
  args?: any; pre_image?: any
  reversible: boolean; status: 'ok' | 'failed' | 'rolled_back'; error?: string
  rolled_back?: { at: string; by: string; error?: string }
}

export const ROLE_PERMS: Record<Role, Set<string>> = {
  viewer:   new Set(['agents.read','system.read','services.read','fs.read','audit.read']),
  operator: new Set(['agents.read','system.read','services.read','services.state','fs.read','fs.write','fs.mkdir','audit.read']),
  admin:    new Set(['agents.read','system.read','services.read','services.state','services.admin','fs.read','fs.write','fs.mkdir','fs.delete','shell.exec','terminal','audit.read','audit.rollback'])
}

export function can(role: Role | null | undefined, action: string): boolean {
  if (!role) return false
  return ROLE_PERMS[role]?.has(action) ?? false
}

export const api = {
  login: (username: string, password: string) =>
    http.post<{ token: string; role: Role; user: string }>('/auth/login', { username, password }).then((r) => r.data),

  me: () => http.get<{ user: string; role: Role }>('/auth/me').then((r) => r.data),

  agents: () => http.get<Agent[]>('/agents').then((r) => r.data ?? []),
  system: (id: string) => http.get<SystemInfo>(`/agents/${id}/system`).then((r) => r.data),

  services: (id: string) =>
    http.get<{ services: Service[] }>(`/agents/${id}/services`).then((r) => r.data?.services ?? []),

  serviceAction: (id: string, name: string, action: string, confirm = true) =>
    http.post(`/agents/${id}/services/${encodeURIComponent(name)}/action`, { action, confirm }).then((r) => r.data),

  fsList: (id: string, path: string) =>
    http.get<{ path: string; entries: FSEntry[] }>(`/agents/${id}/fs/list`, { params: { path } }).then((r) => r.data),

  fsRead: (id: string, path: string) =>
    http.post<{ path: string; content: string; size: number }>(`/agents/${id}/fs/read`, { path }).then((r) => r.data),

  fsWrite: (id: string, path: string, content: string) =>
    http.post(`/agents/${id}/fs/write`, { path, content, confirm: true }).then((r) => r.data),

  fsDelete: (id: string, absPath: string, recursive = false) =>
    http.post(`/agents/${id}/fs/delete`, { path: absPath, recursive, confirm: true, confirm_path: absPath }).then((r) => r.data),

  fsMkdir: (id: string, path: string) =>
    http.post(`/agents/${id}/fs/mkdir`, { path, confirm: true }).then((r) => r.data),

  dispatchTask: (agent_id: string, kind: string, command: string, timeout_sec = 60) =>
    http.post<{ task_id: string; audit_id: string }>('/tasks', { agent_id, kind, command, timeout_sec, confirm: true }).then((r) => r.data),

  taskStreamUrl: (taskId: string) => {
    const t = getToken() ?? ''
    return `/api/tasks/stream?task_id=${encodeURIComponent(taskId)}&access_token=${encodeURIComponent(t)}`
  },

  terminalWS: (id: string) => {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const token = getToken() ?? ''
    return `${proto}//${location.host}/api/agents/${id}/terminal?access_token=${encodeURIComponent(token)}`
  },

  auditList: () => http.get<AuditEntry[]>('/audit').then((r) => r.data ?? []),
  auditGet: (id: string) => http.get<AuditEntry>(`/audit/${id}`).then((r) => r.data),
  auditRollback: (id: string) => http.post<AuditEntry>(`/audit/${id}/rollback`, { confirm: true }).then((r) => r.data)
}
