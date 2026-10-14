<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { cls, dirLabel, num, when } from '../format'

const watches = ref([])
const code = ref('')
const result = ref(null)
const history = ref([])
const error = ref('')
const loading = ref(false)

async function loadHistory() {
  const data = await api('/api/v1/predict/search', { code: code.value, page: 1, size: 12 })
  history.value = data.data || []
}

async function load() {
  const data = await api('/api/v1/watch/list', {})
  watches.value = data.data || []
  if (!code.value && watches.value.length) code.value = watches.value[0].code
  await loadHistory()
}

async function run() {
  if (!code.value) return
  loading.value = true
  error.value = ''
  try {
    result.value = (await api('/api/v1/predict/execute', { code: code.value })).data
    await loadHistory()
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

function pick(next) {
  code.value = next
  result.value = null
  loadHistory()
}

onMounted(load)
</script>

<template>
  <section class="page">
    <div class="page-head">
      <div>
        <h1>涨跌预测</h1>
        <p>用均线、MACD、RSI、5 日动量和量能打分，给出下一交易日的方向倾向。这是技术信号，不是收益承诺。</p>
      </div>
      <button class="primary" :disabled="loading || !code" @click="run">{{ loading ? '计算中' : '生成预测' }}</button>
    </div>
    <p v-if="error" class="banner">{{ error }}</p>
    <div class="chips">
      <button v-for="item in watches" :key="item.code" class="chip" :class="{ on: item.code === code }" @click="pick(item.code)">
        {{ item.name }}
      </button>
    </div>
    <div class="split">
      <article class="card">
        <template v-if="result">
          <div class="spread">
            <h2>{{ result.name }} {{ result.code }}</h2>
            <span class="tag" :class="result.direction === 'up' ? 'up' : result.direction === 'down' ? 'down' : ''">{{ dirLabel(result.direction) }}</span>
          </div>
          <div class="meter"><i class="needle" :style="{ left: `${((result.score + 1) / 2) * 100}%` }"></i></div>
          <div class="spread muted"><span>看跌</span><span>分数 {{ num(result.score, 3) }}</span><span>看涨</span></div>
          <p>置信度 {{ num(result.confidence * 100, 1) }}%，参考收盘价 {{ num(result.close) }}</p>
          <div v-for="reason in result.reasons" :key="reason.name" class="reason">
            <b>{{ reason.name }}</b>
            <div>
              <div class="bar"><i :style="{ width: `${Math.min(100, Math.abs(reason.score) * 100)}%` }" :class="cls(reason.score)"></i></div>
              <span class="muted">{{ reason.detail }} · {{ num(reason.score, 3) }}</span>
            </div>
          </div>
        </template>
        <p v-else class="empty">选择一只股票后生成预测。首次会拉取约 220 根日线。</p>
      </article>
      <article class="card">
        <h2>历史</h2>
        <div v-for="item in history" :key="item.id" class="event">
          <b>{{ item.name }} · {{ dirLabel(item.direction) }}</b>
          <span class="muted">置信 {{ num(item.confidence * 100, 1) }}% · 收盘 {{ num(item.close) }} · {{ when(item.createdAt) }}</span>
        </div>
        <p v-if="!history.length" class="empty">暂无记录。</p>
      </article>
    </div>
  </section>
</template>
