<script setup>
import { onBeforeUnmount, onMounted } from 'vue'
import { store } from './store'
import { cls, num, pct } from './format'

const nav = [
  { to: '/', label: '总览' },
  { to: '/market', label: '行情' },
  { to: '/predict', label: '预测' },
  { to: '/quant', label: '量化' },
  { to: '/monitor', label: '盯盘' },
]

async function refresh() {
  await Promise.allSettled([store.refreshAccount(), store.refreshStatus(), store.refreshSource()])
}

let timer
onMounted(() => {
  refresh()
  timer = setInterval(refresh, 20000)
})
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <div class="shell">
    <aside class="side">
      <div class="brand">
        <span class="mark">衡</span>
        <div>
          <strong>衡观</strong>
          <small>NSTOCK</small>
        </div>
      </div>
      <nav>
        <RouterLink v-for="item in nav" :key="item.to" :to="item.to">{{ item.label }}</RouterLink>
      </nav>
      <div class="side-foot">
        <span>模拟权益</span>
        <b>{{ num(store.account?.equity) }}</b>
        <em :class="cls(store.account?.pnlPct)">{{ pct(store.account?.pnlPct) }}</em>
      </div>
    </aside>
    <section class="main">
      <header class="top">
        <div class="pulse" :class="{ bad: store.status.lastError }">
          <i></i>
          <span>{{ store.status.lastError ? '行情扫描异常' : '盯盘运行中' }}</span>
        </div>
        <span class="source" :class="{ fallback: store.source.fallback }">数据源 · {{ store.source.dataSourceName || '东方财富公开行情' }}</span>
        <span class="disclaimer">预测与交易均为研究模拟，不构成投资建议</span>
      </header>
      <RouterView />
    </section>
  </div>
</template>
