import { reactive } from 'vue'
import { api } from './api'

export const store = reactive({
  account: null,
  status: { running: false, lastError: '', lastScan: null, intervalSeconds: 20 },
  async refreshAccount() {
    this.account = await api('/api/account')
  },
  async refreshStatus() {
    this.status = await api('/api/monitor/status')
  },
})
