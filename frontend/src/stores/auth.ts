import { defineStore } from 'pinia'
import { api, getToken, setToken, getRole, setRole, getUser, setUser, type Role } from '@/api/client'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: getToken() as string | null,
    role: getRole() as Role | null,
    user: getUser() as string | null,
    error: '' as string
  }),
  getters: {
    authed: (s) => !!s.token,
    can: (s) => (action: string) => {
      if (!s.role) return false
      const map: Record<Role, string[]> = {
        viewer:   ['agents.read','system.read','services.read','fs.read','audit.read'],
        operator: ['agents.read','system.read','services.read','services.state','fs.read','fs.write','fs.mkdir','audit.read'],
        admin:    ['agents.read','system.read','services.read','services.state','services.admin','fs.read','fs.write','fs.mkdir','fs.delete','shell.exec','terminal','audit.read','audit.rollback']
      }
      return map[s.role]?.includes(action) ?? false
    }
  },
  actions: {
    async login(username: string, password: string) {
      this.error = ''
      try {
        const { token, role, user } = await api.login(username, password)
        this.token = token; this.role = role; this.user = user
        setToken(token); setRole(role); setUser(user)
      } catch (e: any) {
        this.error = e?.response?.status === 401 ? 'invalid credentials'
                  : e?.response?.status === 429 ? 'rate limited'
                  : 'login failed'
        throw e
      }
    },
    logout() {
      this.token = null; this.role = null; this.user = null
      setToken(null); setRole(null); setUser(null)
    }
  }
})
