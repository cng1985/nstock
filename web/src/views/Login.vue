<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { session } from '../auth'

const route = useRoute()
const router = useRouter()
const username = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

const demos = [
  { username: 'admin', password: 'admin123', label: '管理员', hint: '全部权限' },
  { username: 'trader', password: 'trader123', label: '交易员', hint: '模拟交易' },
  { username: 'analyst', password: 'analyst123', label: '研究员', hint: '只读研究' },
]

async function submit() {
  busy.value = true
  error.value = ''
  try {
    await session.login(username.value.trim(), password.value)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : ''
    if (redirect.startsWith('/') && !redirect.startsWith('//')) router.push(redirect)
    else router.push('/')
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = false
  }
}

function fill(item) {
  username.value = item.username
  password.value = item.password
}
</script>

<template>
  <div class="login-screen">
    <form class="card login-card" @submit.prevent="submit">
      <div class="brand">
        <span class="mark">衡</span>
        <div>
          <strong>衡观</strong>
          <small>NSTOCK</small>
        </div>
      </div>
      <div>
        <h1>登录</h1>
        <p class="note">按角色进入行情、预测、模拟交易和权限管理。这是本地研究终端。</p>
      </div>
      <p v-if="error" class="banner">{{ error }}</p>
      <label>用户名
        <input v-model="username" autocomplete="username" required />
      </label>
      <label>密码
        <input v-model="password" type="password" autocomplete="current-password" required />
      </label>
      <button class="primary" type="submit" :disabled="busy">{{ busy ? '登录中…' : '进入终端' }}</button>
      <div class="demo">
        <span>初始账号，登录后可在侧栏修改密码</span>
        <button v-for="item in demos" :key="item.username" type="button" class="ghost" @click="fill(item)">
          <b>{{ item.label }}</b>
          <em>{{ item.username }} / {{ item.password }}</em>
          <span>{{ item.hint }}</span>
        </button>
      </div>
    </form>
  </div>
</template>
