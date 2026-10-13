<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import KlineChart from '../components/KlineChart.vue'
import { api } from '../api'
import { cls, compact, dirLabel, marketLabel, num, pct, signed } from '../format'
import { store } from '../store'

const route = useRoute()
const quote = ref(null)
const kline = ref(null)
const prediction = ref(null)
const watches = ref([])
const qty = ref(100)
const alertKind = ref('price_above')
const alertThreshold = ref(0)
const error = ref('')
const message = ref('')
const busy = ref('')

const code = computed(() => route.params.code)
const watched = computed(() => watches.value.some((item) => item.code === code.value))

async function load() {
  error.value = ''
  message.value = ''
  prediction.value = null
  const [q, k, w] = await Promise.allSettled([
    api(`/api/market/quote/${code.value}?refresh=1`),
    api(`/api/market/kline/${code.value}?limit=180`),
    api('/api/watchlist'),
  ])
  if (q.status === 'fulfilled') quote.value = q.value
  else error.value = q.reason.message
  if (k.status === 'fulfilled') kline.value = k.value
  else if (!error.value) error.value = k.reason.message
  if (w.status === 'fulfilled') watches.value = w.value.items || []
  if (!alertThreshold.value && quote.value?.price) alertThreshold.value = quote.value.price
}

async function toggleWatch() {
  if (watched.value) {
    await api(`/api/watchlist/${code.value}`, { method: 'DELETE' })
  } else {
    await api('/api/watchlist', { method: 'POST', body: JSON.stringify({ code: code.value }) })
  }
  await load()
}

async function predict() {
  busy.value = 'predict'
  error.value = ''
  try {
    prediction.value = await api(`/api/predict/${code.value}`, { method: 'POST' })
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function trade(side) {
  busy.value = side
  error.value = ''
  message.value = ''
  try {
    const order = await api('/api/orders', {
      method: 'POST',
      body: JSON.stringify({ code: code.value, side, qty: Number(qty.value) }),
    })
    message.value = `${order.message}，价格 ${num(order.price)}`
    await store.refreshAccount()
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function createAlert() {
  busy.value = 'alert'
  error.value = ''
  try {
    await api('/api/alerts', {
      method: 'POST',
      body: JSON.stringify({ code: code.value, kind: alertKind.value, threshold: Number(alertThreshold.value) }),
    })
    message.value = '盯盘条件已保存'
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

onMounted(load)
watch(code, load)
</script>

<template>
  <section class="page">
    <div class="page-head">
      <div>
        <h1>{{ quote?.name || kline?.name || code }} <span class="muted">{{ marketLabel(quote?.market || kline?.market) }} {{ code }}</span></h1>
        <p class="price" :class="cls(quote?.changePct)">{{ num(quote?.price) }} <small>{{ signed(quote?.change) }} {{ pct(quote?.changePct) }}</small></p>
      </div>
      <div class="actions">
        <button @click="toggleWatch">{{ watched ? '移出自选' : '加入自选' }}</button>
        <button :disabled="busy === 'predict'" @click="predict">生成预测</button>
      </div>
    </div>
    <p v-if="error" class="banner">{{ error }}</p>
    <p v-if="message" class="banner warn">{{ message }}</p>
    <div class="split">
      <article class="card">
        <KlineChart v-if="kline?.bars?.length" :bars="kline.bars" />
        <p v-else class="empty">暂无 K 线。</p>
      </article>
      <div class="stack">
        <article class="card kvs">
          <div><span class="muted">开盘</span><b class="num">{{ num(quote?.open) }}</b></div>
          <div><span class="muted">最高</span><b class="num">{{ num(quote?.high) }}</b></div>
          <div><span class="muted">最低</span><b class="num">{{ num(quote?.low) }}</b></div>
          <div><span class="muted">昨收</span><b class="num">{{ num(quote?.preClose) }}</b></div>
          <div><span class="muted">成交量(手)</span><b class="num">{{ compact(quote?.volume) }}</b></div>
          <div><span class="muted">成交额</span><b class="num">{{ compact(quote?.amount) }}</b></div>
        </article>
        <article class="card">
          <h2>模拟下单</h2>
          <p class="note">按最新价立即成交，100 股为一手，不会发送到券商。</p>
          <div class="actions">
            <input v-model.number="qty" type="number" min="100" step="100" style="max-width: 140px" />
            <button class="primary" :disabled="busy === 'buy'" @click="trade('buy')">买入</button>
            <button :disabled="busy === 'sell'" @click="trade('sell')">卖出</button>
          </div>
        </article>
        <article class="card form">
          <h2>盯盘</h2>
          <label>条件
            <select v-model="alertKind">
              <option value="price_above">价格高于</option>
              <option value="price_below">价格低于</option>
              <option value="pct_above">涨幅达到 %</option>
              <option value="pct_below">跌幅达到 %</option>
            </select>
          </label>
          <label>阈值
            <input v-model.number="alertThreshold" type="number" step="0.01" />
          </label>
          <button :disabled="busy === 'alert'" @click="createAlert">保存条件</button>
        </article>
        <article v-if="prediction" class="card">
          <h2>{{ dirLabel(prediction.direction) }}</h2>
          <p class="note">下一交易日倾向，置信 {{ num(prediction.confidence * 100, 1) }}%</p>
          <RouterLink to="/predict">查看依据</RouterLink>
        </article>
      </div>
    </div>
  </section>
</template>
