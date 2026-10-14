<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import { cls, marketLabel, num, pct } from '../format'
import { store } from '../store'

const router = useRouter()
const query = ref('')
const hits = ref([])
const quotes = ref({ items: [] })
const error = ref('')
const searching = ref(false)
let timer

async function load() {
  const res = await api('/api/v1/quote/list', { refresh: true })
  quotes.value = res.data
  await store.refreshSource()
}

async function search() {
  if (!query.value.trim()) {
    hits.value = []
    return
  }
  searching.value = true
  error.value = ''
  try {
    const data = await api('/api/v1/market/search', { keyword: query.value.trim() })
    hits.value = data.data || []
  } catch (err) {
    error.value = err.message
  } finally {
    searching.value = false
  }
}

function schedule() {
  clearTimeout(timer)
  timer = setTimeout(search, 280)
}

async function add(code) {
  const value = String(code || '').trim()
  if (!value) return
  error.value = ''
  try {
    await api('/api/v1/watch/create', { code: value })
    query.value = ''
    hits.value = []
    await load()
  } catch (err) {
    error.value = err.message
  }
}

async function remove(code) {
  if (!window.confirm(`从自选移除 ${code}？`)) return
  await api('/api/v1/watch/delete', { code })
  await load()
}

onMounted(load)
</script>

<template>
  <section class="page">
    <div class="page-head">
      <div>
        <h1>行情</h1>
        <p>搜索代码或名称加入自选。东方财富不可用时会自动改用启动数据源，自选和日线仍可继续。</p>
      </div>
    </div>
    <p v-if="error" class="banner">{{ error }}</p>
    <p v-if="quotes.warning" class="banner warn">{{ quotes.warning }}</p>
    <article class="card">
      <div class="add-row">
        <label>代码或名称
          <input v-model="query" placeholder="例如 600519 或 茅台" @input="schedule" @keyup.enter="add(query)" />
        </label>
        <button class="primary" @click="add(query)">加入自选</button>
      </div>
      <p v-if="searching" class="muted">正在搜索…</p>
      <table v-if="hits.length">
        <tbody>
          <tr v-for="hit in hits" :key="hit.secId">
            <td>{{ hit.name }} <span class="muted">{{ marketLabel(hit.market) }} {{ hit.code }}</span></td>
            <td class="muted nowrap">{{ hit.kindName }}</td>
            <td class="right">
              <button class="ghost" @click="router.push(`/quote/${hit.code}`)">查看</button>
              <button class="primary" @click="add(hit.code)">加入自选</button>
            </td>
          </tr>
        </tbody>
      </table>
    </article>
    <article class="card">
      <h2>自选列表</h2>
      <table>
        <thead>
          <tr>
            <th>证券</th>
            <th class="right">最新</th>
            <th class="right">涨跌幅</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in quotes.items" :key="item.code">
            <td>{{ item.name }} <span class="muted">{{ marketLabel(item.market) }} {{ item.code }}</span></td>
            <td class="num" :class="cls(item.quote?.changePct)">{{ num(item.quote?.price) }}</td>
            <td class="num" :class="cls(item.quote?.changePct)">{{ pct(item.quote?.changePct) }}</td>
            <td class="right">
              <button class="ghost" @click="router.push(`/quote/${item.code}`)">详情</button>
              <button class="danger" @click="remove(item.code)">移除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </article>
  </section>
</template>
