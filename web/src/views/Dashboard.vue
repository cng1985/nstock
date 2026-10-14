<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import { cls, compact, dirLabel, marketLabel, num, pct, signed, when } from '../format'
import { store } from '../store'

const router = useRouter()
const indices = ref([])
const quotes = ref({ items: [] })
const events = ref([])
const predictions = ref([])
const error = ref('')
const loading = ref(false)

async function load() {
  loading.value = true
  error.value = ''
  const [idx, list, ev, pred] = await Promise.allSettled([
    api('/api/v1/market/indices', {}),
    api('/api/v1/quote/list', { refresh: true }),
    api('/api/v1/alertEvent/search', { page: 1, size: 6 }),
    api('/api/v1/predict/search', { page: 1, size: 6 }),
  ])
  if (idx.status === 'fulfilled') indices.value = idx.value.data || []
  else error.value = idx.reason.message
  if (list.status === 'fulfilled') quotes.value = list.value.data
  else if (!error.value) error.value = list.reason.message
  if (ev.status === 'fulfilled') events.value = ev.value.data || []
  if (pred.status === 'fulfilled') predictions.value = pred.value.data || []
  await store.refreshSource()
  loading.value = false
}

onMounted(load)
</script>

<template>
  <section class="page">
    <div class="page-head">
      <div>
        <h1>总览</h1>
        <p>指数、自选和模拟账户放在同一张桌上。红涨绿跌。</p>
      </div>
      <button :disabled="loading" @click="load">刷新行情</button>
    </div>
    <p v-if="error" class="banner">{{ error }}</p>
    <p v-if="quotes.warning" class="banner warn">{{ quotes.warning }}</p>

    <div class="grid-4">
      <article v-for="item in indices" :key="item.secId" class="card index">
        <span>{{ item.name }}</span>
        <strong :class="cls(item.changePct)">{{ num(item.price) }}</strong>
        <em :class="cls(item.changePct)">{{ signed(item.change) }} {{ pct(item.changePct) }}</em>
      </article>
    </div>

    <div class="split">
      <article class="card">
        <div class="spread">
          <h2>自选</h2>
          <RouterLink to="/market">管理</RouterLink>
        </div>
        <table>
          <thead>
            <tr>
              <th>名称</th>
              <th class="right">最新</th>
              <th class="right">涨跌</th>
              <th class="right">成交额</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in quotes.items" :key="item.code" class="link" @click="router.push(`/quote/${item.code}`)">
              <td>
                {{ item.name }}
                <span class="muted">{{ marketLabel(item.market) }} {{ item.code }}</span>
              </td>
              <td class="num" :class="cls(item.quote?.changePct)">{{ num(item.quote?.price) }}</td>
              <td class="num" :class="cls(item.quote?.changePct)">{{ pct(item.quote?.changePct) }}</td>
              <td class="num">{{ compact(item.quote?.amount) }}</td>
            </tr>
          </tbody>
        </table>
        <p v-if="!quotes.items.length" class="empty">自选还是空的。</p>
      </article>
      <div class="stack">
        <article class="card">
          <h2>模拟账户</h2>
          <div class="kvs">
            <div class="stat"><span>权益</span><strong>{{ num(store.account?.equity) }}</strong></div>
            <div class="stat"><span>可用</span><strong>{{ num(store.account?.cash) }}</strong></div>
            <div class="stat"><span>市值</span><strong>{{ num(store.account?.marketValue) }}</strong></div>
            <div class="stat"><span>盈亏</span><strong :class="cls(store.account?.pnl)">{{ pct(store.account?.pnlPct) }}</strong></div>
          </div>
        </article>
        <article class="card">
          <h2>最近预测</h2>
          <div v-for="item in predictions" :key="item.id" class="event">
            <b>{{ item.name }} · {{ dirLabel(item.direction) }}</b>
            <span class="muted">置信 {{ num((item.confidence || 0) * 100, 1) }}% · {{ when(item.createdAt) }}</span>
          </div>
          <p v-if="!predictions.length" class="empty">还没有预测记录。到预测页生成第一份。</p>
        </article>
        <article class="card">
          <h2>盯盘提醒</h2>
          <div v-for="item in events" :key="item.id" class="event">
            <b>{{ item.message }}</b>
            <span class="muted">{{ when(item.createdAt) }}</span>
          </div>
          <p v-if="!events.length" class="empty">还没有触发记录。</p>
        </article>
      </div>
    </div>
  </section>
</template>
