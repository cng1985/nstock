<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from './api'
import { session } from './auth'
import { cls, num, pct } from './format'
import { store } from './store'

const route = useRoute()
const router = useRouter()
const nav = [
  { to: '/', label: '总览', any: ['stock:quote:view', 'stock:account:view', 'stock:predict:view'] },
  { to: '/market', label: '行情', any: ['stock:quote:view'] },
  { to: '/predict', label: '预测', any: ['stock:predict:view', 'stock:predict:execute'] },
  { to: '/quant', label: '量化', any: ['stock:strategy:view', 'stock:order:view', 'stock:account:view'] },
  { to: '/monitor', label: '盯盘', any: ['stock:alert:view', 'stock:monitor:view'] },
  { to: '/access', label: '权限', any: ['stock:user:view', 'stock:role:view'] },
]
const visibleNav = computed(() => nav.filter((item) => session.can(...item.any)))
const roleLabel = computed(() => (session.user?.roles || []).map((item) => item.name).join('、') || '未分配角色')
const showPassword = ref(false)
const oldPassword = ref('')
const newPassword = ref('')
const passError = ref('')
const passMessage = ref('')

async function refresh() {
  await Promise.allSettled([store.refreshAccount(), store.refreshStatus(), store.refreshSource()])
}

let timer
onMounted(() => {
  refresh()
  timer = setInterval(refresh, 20000)
})
onBeforeUnmount(() => clearInterval(timer))
watch(() => session.user?.id, refresh)
</script>

<template>
  <RouterView v-if="route.meta.public" />
  <div v-else class="shell">
    <aside class="side">
      <div class="brand">
        <span class="mark">衡</span>
        <div>
          <strong>衡观</strong>
          <small>NSTOCK</small>
        </div>
      </div>
      <nav>
        <RouterLink v-for="item in visibleNav" :key="item.to" :to="item.to">{{ item.label }}</RouterLink>
      </nav>
      <div v-if="session.can('stock:account:view')" class="side-foot">
        <span>模拟权益</span>
        <b>{{ num(store.account?.equity) }}</b>
        <em :class="cls(store.account?.pnlPct)">{{ pct(store.account?.pnlPct) }}</em>
      </div>
      <div class="side-user" :class="{ 'at-end': !session.can('stock:account:view') }">
        <b>{{ session.user?.displayName }}</b>
        <span class="muted">{{ session.user?.username }} · {{ roleLabel }}</span>
        <button v-if="session.user?.id" class="ghost" type="button" @click="showPassword = !showPassword">修改密码</button>
        <form v-if="showPassword" class="side-pass" @submit.prevent="changePassword">
          <input v-model="oldPassword" type="password" placeholder="原密码" autocomplete="current-password" />
          <input v-model="newPassword" type="password" placeholder="新密码" autocomplete="new-password" />
          <button class="primary" type="submit">保存</button>
          <p v-if="passError" class="banner">{{ passError }}</p>
          <p v-if="passMessage" class="banner warn">{{ passMessage }}</p>
        </form>
        <button class="ghost" type="button" @click="logout">退出登录</button>
      </div>
    </aside>
    <section class="main">
      <header class="top">
        <div v-if="session.can('stock:monitor:view')" class="pulse" :class="{ bad: store.status.lastError }">
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
