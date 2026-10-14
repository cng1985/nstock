import { reactive } from 'vue'
import { api } from './api'

export const store = reactive({
  account: null,
  status: { running: false, lastError: '', lastScan: null, intervalSeconds: 20 },
  source: { dataSource: 'eastmoney', dataSourceName: '东方财富公开行情', fallback: false },
  async refreshAccount() {
    const res = await api('/api/v1/account/get', {})
    this.account = res.data
  },
  async refreshStatus() {
    const res = await api('/api/v1/monitor/get', {})
    this.status = res.data
  },
  async refreshSource() {
    try {
      const res = await api('/api/v1/system/health', {})
      this.source = res.data
    } catch {
      // 健康检查失败时保留上一次的数据源状态，页面行情请求仍单独报错。
    }
  },
})
