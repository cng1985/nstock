import { reactive } from 'vue'
import { api, getToken, setToken } from './api'

export const session = reactive({
  ready: false,
  user: null,
  async login(username, password) {
    const res = await api('/api/v1/auth/login', { username, password })
    setToken(res.data.token)
    this.user = res.data.user
    this.ready = true
    return this.user
  },
  async logout() {
    try {
      if (getToken()) await api('/api/v1/auth/logout', {})
    } catch {
      // 会话失效时仍清除本地登录态。
    }
    setToken('')
    this.user = null
  },
  async restore() {
    if (this.ready) return
    if (!getToken()) {
      this.user = null
      this.ready = true
      return
    }
    try {
      const res = await api('/api/v1/auth/me', {})
      this.user = res.data
    } catch {
      this.user = null
      setToken('')
    }
    this.ready = true
  },
  can(...codes) {
    const have = new Set(this.user?.permissions || [])
    return codes.some((code) => have.has(code))
  },
})
