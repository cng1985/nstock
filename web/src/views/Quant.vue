<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import EquityChart from '../components/EquityChart.vue'
import { api } from '../api'
import { session } from '../auth'
import { cls, kindLabel, num, pct, sideLabel, signalLabel, when } from '../format'
import { store } from '../store'

const types = [
  { id: 'ma_cross', desc: '短均线高于长均线时持有，否则空仓。' },
  { id: 'rsi', desc: 'RSI 低于超卖线买入，高于超买线卖出。' },
  { id: 'momentum', desc: '近 N 日涨幅超过阈值且价格在均线之上时持有。' },
]

const watches = ref([])
const strategies = ref([])
const currentId = ref(0)
const report = ref(null)
const orders = ref([])
const error = ref('')
const message = ref('')
const busy = ref('')
const manual = reactive({ code: '', side: 'buy', qty: 100 })
const form = reactive({
  name: '',
  type: 'ma_cross',
  code: '',
  fast: 5,
  slow: 20,
  rsiPeriod: 14,
  oversold: 30,
  overbought: 70,
  lookback: 10,
  threshold: 0.03,
  maPeriod: 20,
  positionPct: 0.3,
})

const current = computed(() => strategies.value.find((item) => item.id === currentId.value))

function paramsOf(source) {
  const positionPct = Number(source.positionPct)
  if (source.type === 'rsi') {
    return { rsiPeriod: Number(source.rsiPeriod), oversold: Number(source.oversold), overbought: Number(source.overbought), positionPct }
  }
  if (source.type === 'momentum') {
    return { lookback: Number(source.lookback), threshold: Number(source.threshold), maPeriod: Number(source.maPeriod), positionPct }
  }
  return { fast: Number(source.fast), slow: Number(source.slow), positionPct }
}

async function load() {
  const [watch, list, blotter] = await Promise.allSettled([
    api('/api/v1/watch/list', {}),
    api('/api/v1/strategy/list', {}),
    api('/api/v1/order/search', { page: 1, size: 20 }),
  ])
  if (watch.status === 'fulfilled') watches.value = watch.value.data || []
  if (list.status === 'fulfilled') strategies.value = list.value.data || []
  if (blotter.status === 'fulfilled') orders.value = blotter.value.data || []
  if (!form.code && watches.value.length) form.code = watches.value[0].code
  if (!manual.code && watches.value.length) manual.code = watches.value[0].code
  if (!currentId.value && strategies.value.length) select(strategies.value[0])
}

function select(item) {
  currentId.value = item.id
  report.value = null
  Object.assign(form, item.params || {}, { type: item.type, code: item.code, name: item.name })
  api('/api/v1/backtest/search', { strategyId: item.id, page: 1, size: 1 }).then((data) => {
    report.value = data.data?.[0] || null
  }).catch(() => {})
}

async function create() {
  busy.value = 'create'
  error.value = ''
  try {
    const created = await api('/api/v1/strategy/create', {
      name: form.name, type: form.type, code: form.code, params: paramsOf(form),
    })
    await load()
    select(created.data)
    message.value = '策略已创建'
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function saveParams() {
  if (!current.value) return
  busy.value = 'save'
  error.value = ''
  try {
    await api('/api/v1/strategy/update', {
      id: current.value.id,
      name: form.name,
      code: form.code,
      params: paramsOf({ ...form, type: current.value.type }),
    })
    await load()
    message.value = '参数已保存'
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function toggle() {
  if (!current.value) return
  await api('/api/v1/strategy/update', { id: current.value.id, enabled: !current.value.enabled })
  await load()
}

async function remove() {
  if (!current.value || !window.confirm('删除这个策略？')) return
  await api('/api/v1/strategy/delete', { id: current.value.id })
  currentId.value = 0
  report.value = null
  await load()
}

async function backtest() {
  if (!current.value) return
  busy.value = 'backtest'
  error.value = ''
  message.value = ''
  try {
    report.value = (await api('/api/v1/strategy/backtest', { id: current.value.id })).data
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function run(id) {
  busy.value = 'run'
  error.value = ''
  try {
    const data = id
      ? await api('/api/v1/strategy/execute', { id })
      : await api('/api/v1/strategy/executeEnabled', {})
    message.value = id ? data.data.message : (data.data || []).map((item) => `${item.name}：${item.message}`).join('；')
    await Promise.all([load(), store.refreshAccount()])
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function place() {
  busy.value = 'order'
  error.value = ''
  try {
    const order = await api('/api/v1/order/create', {
      code: manual.code, side: manual.side, qty: Number(manual.qty),
    })
    message.value = order.data.message
    await Promise.all([load(), store.refreshAccount()])
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function reset() {
  if (!window.confirm('清空模拟持仓和委托，现金恢复到初始资金？策略会保留。')) return
  await api('/api/v1/account/reset', {})
  await Promise.all([load(), store.refreshAccount()])
  message.value = '模拟账户已重置'
}

onMounted(load)
</script>

<template>
  <section class="page">
    <div class="page-head">
      <div>
        <h1>量化交易</h1>
        <p>回测和执行都在模拟账户里完成。成交价使用日线收盘或当前最新价，包含佣金和卖出印花税，只做多。</p>
      </div>
      <div class="actions">
        <button v-if="session.can('stock:strategy:execute')" :disabled="busy === 'run'" @click="run()">执行已启用策略</button>
        <button v-if="session.can('stock:account:reset')" class="danger" @click="reset">重置账户</button>
      </div>
    </div>
    <p v-if="error" class="banner">{{ error }}</p>
    <p v-if="message" class="banner warn">{{ message }}</p>

    <div class="split">
      <div class="stack">
        <article v-if="session.can('stock:strategy:create')" class="card form">
          <h2>新建策略</h2>
          <label>名称<input v-model="form.name" placeholder="可留空，自动命名" /></label>
          <label>类型
            <select v-model="form.type">
              <option v-for="item in types" :key="item.id" :value="item.id">{{ kindLabel(item.id) }}</option>
            </select>
          </label>
          <p class="note">{{ types.find((item) => item.id === form.type)?.desc }}</p>
          <label>证券
            <input v-model="form.code" list="codes" placeholder="600519" />
          </label>
          <div class="grid-3">
            <template v-if="form.type === 'ma_cross'">
              <label>短周期<input v-model.number="form.fast" type="number" /></label>
              <label>长周期<input v-model.number="form.slow" type="number" /></label>
            </template>
            <template v-else-if="form.type === 'rsi'">
              <label>RSI 周期<input v-model.number="form.rsiPeriod" type="number" /></label>
              <label>超卖<input v-model.number="form.oversold" type="number" /></label>
              <label>超买<input v-model.number="form.overbought" type="number" /></label>
            </template>
            <template v-else>
              <label>回看天数<input v-model.number="form.lookback" type="number" /></label>
              <label>涨幅阈值<input v-model.number="form.threshold" type="number" step="0.01" /></label>
              <label>均线<input v-model.number="form.maPeriod" type="number" /></label>
            </template>
            <label>仓位比例<input v-model.number="form.positionPct" type="number" min="0.05" max="1" step="0.05" /></label>
          </div>
          <button class="primary" :disabled="busy === 'create'" @click="create">创建</button>
        </article>
        <article class="card">
          <h2>策略</h2>
          <button v-for="item in strategies" :key="item.id" class="strategy" :class="{ on: item.id === currentId }" @click="select(item)">
            <b>{{ item.name }}</b>
            <span class="muted">{{ item.code }} · {{ item.enabled ? '启用' : '停用' }} · {{ signalLabel(item.lastSignal) }}</span>
          </button>
          <p v-if="!strategies.length" class="empty">还没有策略。</p>
        </article>
      </div>

      <div class="stack">
        <article v-if="current" class="card stack">
          <div class="spread">
            <h2>{{ current.name }}</h2>
            <span class="tag" :class="{ on: current.enabled }">{{ current.enabled ? '启用' : '停用' }}</span>
          </div>
          <p class="note">{{ current.lastMessage || '尚未执行' }} <span v-if="current.lastRunAt">· {{ when(current.lastRunAt) }}</span></p>
          <div class="actions">
            <button v-if="session.can('stock:strategy:update')" @click="toggle">{{ current.enabled ? '停用' : '启用' }}</button>
            <button v-if="session.can('stock:strategy:update')" :disabled="busy === 'save'" @click="saveParams">保存参数</button>
            <button v-if="session.can('stock:strategy:backtest')" :disabled="busy === 'backtest'" @click="backtest">回测</button>
            <button v-if="session.can('stock:strategy:execute')" class="primary" :disabled="busy === 'run'" @click="run(current.id)">执行一次</button>
            <button v-if="session.can('stock:strategy:delete')" class="danger" @click="remove">删除</button>
          </div>
          <template v-if="report">
            <div class="kvs">
              <div class="stat"><span>收益率</span><strong :class="cls(report.returnPct)">{{ pct(report.returnPct) }}</strong></div>
              <div class="stat"><span>期末权益</span><strong>{{ num(report.finalEquity) }}</strong></div>
              <div class="stat"><span>最大回撤</span><strong>{{ num(report.maxDrawdown) }}%</strong></div>
              <div class="stat"><span>胜率</span><strong>{{ num(report.winRate) }}%</strong></div>
            </div>
            <p class="note">{{ report.trades }} 笔成交。{{ report.note }}</p>
            <EquityChart v-if="report.curve?.length" :curve="report.curve" />
          </template>
        </article>
        <article v-if="session.can('stock:order:create')" class="card form">
          <h2>手动模拟单</h2>
          <div class="form inline">
            <label>证券<input v-model="manual.code" list="codes" /></label>
            <label>方向
              <select v-model="manual.side">
                <option value="buy">买入</option>
                <option value="sell">卖出</option>
              </select>
            </label>
            <label>股数<input v-model.number="manual.qty" type="number" min="100" step="100" /></label>
            <button :disabled="busy === 'order'" @click="place">下单</button>
          </div>
        </article>
      </div>
    </div>

    <div class="split even">
      <article class="card">
        <h2>持仓</h2>
        <table>
          <thead>
            <tr><th>证券</th><th class="right">数量</th><th class="right">成本</th><th class="right">最新</th><th class="right">盈亏</th></tr>
          </thead>
          <tbody>
            <tr v-for="item in store.account?.positions || []" :key="item.code">
              <td>{{ item.name }} {{ item.code }}</td>
              <td class="num">{{ item.qty }}</td>
              <td class="num">{{ num(item.avgCost) }}</td>
              <td class="num">{{ num(item.last) }}</td>
              <td class="num" :class="cls(item.pnl)">{{ num(item.pnl) }}</td>
            </tr>
          </tbody>
        </table>
        <p v-if="!store.account?.positions?.length" class="empty">空仓。</p>
      </article>
      <article class="card">
        <h2>委托</h2>
        <table>
          <thead>
            <tr><th>时间</th><th>证券</th><th>方向</th><th class="right">价格</th><th>状态</th></tr>
          </thead>
          <tbody>
            <tr v-for="item in orders" :key="item.id">
              <td>{{ when(item.createdAt) }}</td>
              <td>{{ item.name }}</td>
              <td>{{ sideLabel(item.side) }} {{ item.qty }}</td>
              <td class="num">{{ num(item.price) }}</td>
              <td>{{ item.status === 'filled' ? '成交' : '拒绝' }}</td>
            </tr>
          </tbody>
        </table>
      </article>
    </div>
    <datalist id="codes">
      <option v-for="item in watches" :key="item.code" :value="item.code">{{ item.name }}</option>
    </datalist>
  </section>
</template>
