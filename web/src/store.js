import { reactive } from 'vue'
import { api } from './api'

export const store = reactive({
  account: null,
  status: { running: false, lastError: '', lastScan: null, intervalSeconds: 20 },
  async refreshAccount() {
    const res = await api('/api/v1/account/get', {})
    this.account = res.data
  },
  async refreshStatus() {
    const res = await api('/api/v1/monitor/get', {})
    this.status = res.data
  },
})
