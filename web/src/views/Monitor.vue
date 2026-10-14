<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { api } from '../api'
import { session } from '../auth'
import { kindLabel, num, when } from '../format'
import { store } from '../store'

const watches = ref([])
const rules = ref([])
const events = ref([])
const error = ref('')
const message = ref('')
const busy = ref(false)
const form = reactive({ code: '', kind: 'price_above', threshold: 0, name: '' })

async function load() {
  const [watch, ruleData, eventData] = await Promise.allSettled([
    api('/api/v1/watch/list', {}),
    api('/api/v1/alert/list', {}),
    api('/api/v1/alertEvent/search', { page: 1, size: 30 }),
  ])
  if (watch.status === 'fulfilled') watches.value = watch.value.data || []
  if (ruleData.status === 'fulfilled') rules.value = ruleData.value.data || []
  if (eventData.status === 'fulfilled') events.value = eventData.value.data || []
  if (!form.code && watches.value.length) form.code = watches.value[0].code
  await store.refreshStatus()
}

async function create() {
  error.value = ''
  message.value = ''
  try {
    await api('/api/v1/alert/create', {
      code: form.code,
      kind: form.kind,
      threshold: Number(form.threshold),
      name: form.name,
    })
    form.name = ''
    message.value = '盯盘条件已添加'
    await load()
  } catch (err) {
    error.value = err.message
  }
}

async function toggle(rule) {
  await api('/api/v1/alert/update', { id: rule.id, enabled: !rule.enabled })
  await load()
}

async function remove(rule) {
  if (!window.confirm(`删除「${rule.name}」？`)) return
  await api('/api/v1/alert/delete', { id: rule.id })
  await load()
}

async function scan() {
  busy.value = true
  error.value = ''
  try {
    const result = await api('/api/v1/monitor/scan', {})
    message.value = `已检查 ${result.data.checked} 条，新触发 ${result.data.fired} 条`
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = false
  }
}

let timer
onMounted(() => {
  load()
  timer = setInterval(load, 15000)
})
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <section class="page">
    <div class="page-head">
      <div>
        <h1>盯盘</h1>
        <p>后台每 {{ store.status.intervalSeconds }} 秒扫描自选和条件。触发后先锁住，价格离开条件才会再次提醒。</p>
      </div>
      <button v-if="session.can('stock:monitor:execute')" class="primary" :disabled="busy" @click="scan">立即扫描</button>
    </div>
    <p v-if="error" class="banner">{{ error }}</p>
    <p v-if="message" class="banner warn">{{ message }}</p>
    <p v-if="store.status.lastError" class="banner">最近扫描：{{ store.status.lastError }}</p>
    <p v-else-if="store.status.lastScan" class="note">最近扫描 {{ when(store.status.lastScan) }}，检查 {{ store.status.checked }} 条。</p>

    <article v-if="session.can('stock:alert:create')" class="card">
      <form class="form inline" @submit.prevent="create">
        <label>证券
          <input v-model="form.code" list="watch-codes" required />
        </label>
        <label>条件
          <select v-model="form.kind">
            <option value="price_above">价格高于</option>
            <option value="price_below">价格低于</option>
            <option value="pct_above">涨幅达到 %</option>
            <option value="pct_below">跌幅达到 %</option>
          </select>
        </label>
        <label>阈值
          <input v-model.number="form.threshold" type="number" step="0.01" required />
        </label>
        <button class="primary" type="submit">添加</button>
      </form>
      <p class="note">跌幅填写 2，表示跌幅达到 2% 时提醒。价格条件填写绝对价格。</p>
    </article>

    <div class="split">
      <article class="card">
        <h2>条件</h2>
        <table>
          <thead>
            <tr><th>证券</th><th>条件</th><th class="right">现价</th><th>状态</th><th></th></tr>
          </thead>
          <tbody>
            <tr v-for="rule in rules" :key="rule.id">
              <td>{{ rule.stockName }} <span class="muted">{{ rule.code }}</span></td>
              <td>{{ rule.name || kindLabel(rule.kind) }}</td>
              <td class="num">{{ num(rule.lastPrice) }}</td>
              <td>{{ rule.enabled ? (rule.latched ? '已触发' : '监视中') : '停用' }}</td>
              <td class="right">
                <button v-if="session.can('stock:alert:update')" class="ghost" @click="toggle(rule)">{{ rule.enabled ? '停用' : '启用' }}</button>
                <button v-if="session.can('stock:alert:delete')" class="danger" @click="remove(rule)">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
        <p v-if="!rules.length" class="empty">还没有盯盘条件。</p>
      </article>
      <article class="card">
        <h2>提醒</h2>
        <div v-for="item in events" :key="item.id" class="event">
          <b>{{ item.message }}</b>
          <span class="muted">{{ when(item.createdAt) }} · 现价 {{ num(item.price) }}</span>
        </div>
        <p v-if="!events.length" class="empty">条件触发后会出现在这里。</p>
      </article>
    </div>
    <datalist id="watch-codes">
      <option v-for="item in watches" :key="item.code" :value="item.code">{{ item.name }}</option>
    </datalist>
  </section>
</template>
