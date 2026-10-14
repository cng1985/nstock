<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '../api'
import { session } from '../auth'

const tab = ref('users')
const users = ref([])
const roles = ref([])
const permissions = ref([])
const error = ref('')
const message = ref('')
const busy = ref('')
const editingId = ref(0)
const selectedRoleId = ref(0)
const draftPerms = ref([])

const createUser = reactive({ username: '', displayName: '', password: '', roleIds: [], enabled: true })
const editUser = reactive({ displayName: '', password: '', enabled: true, roleIds: [] })
const createRole = reactive({ code: '', name: '', description: '' })
const editRole = reactive({ name: '', description: '' })

const showUsers = computed(() => session.can('stock:user:view'))
const showRoles = computed(() => session.can('stock:role:view'))
const selectedRole = computed(() => roles.value.find((item) => item.id === selectedRoleId.value))
const editingUser = computed(() => users.value.find((item) => item.id === editingId.value))
const grouped = computed(() => {
  const order = []
  const map = new Map()
  for (const item of permissions.value) {
    if (!map.has(item.group)) {
      map.set(item.group, [])
      order.push(item.group)
    }
    map.get(item.group).push(item)
  }
  return order.map((name) => ({ name, items: map.get(name) }))
})

function roleNames(user) {
  return (user.roles || []).map((item) => item.name).join('、') || '未分配'
}

async function load() {
  error.value = ''
  const jobs = []
  if (showUsers.value) jobs.push(api('/api/v1/user/list', {}).then((res) => { users.value = res.data || [] }))
  if (showRoles.value || showUsers.value) jobs.push(api('/api/v1/role/list', {}).then((res) => { roles.value = res.data || [] }))
  if (showRoles.value) jobs.push(api('/api/v1/permission/list', {}).then((res) => { permissions.value = res.data || [] }))
  const result = await Promise.allSettled(jobs)
  const failed = result.find((item) => item.status === 'rejected')
  if (failed) error.value = failed.reason.message
  if (!selectedRoleId.value && roles.value.length) selectRole(roles.value[0])
  if (showUsers.value) tab.value = 'users'
  else tab.value = 'roles'
}

function edit(user) {
  editingId.value = user.id
  editUser.displayName = user.displayName
  editUser.password = ''
  editUser.enabled = user.enabled
  editUser.roleIds = (user.roles || []).map((item) => item.id)
}

async function addUser() {
  busy.value = 'user'
  error.value = ''
  message.value = ''
  try {
    await api('/api/v1/user/create', {
      username: createUser.username,
      displayName: createUser.displayName,
      password: createUser.password,
      enabled: createUser.enabled,
      roleIds: createUser.roleIds,
    })
    createUser.username = ''
    createUser.displayName = ''
    createUser.password = ''
    createUser.roleIds = []
    message.value = '用户已创建'
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function saveUser() {
  busy.value = 'save-user'
  error.value = ''
  message.value = ''
  try {
    const body = {
      id: editingId.value,
      displayName: editUser.displayName,
      enabled: editUser.enabled,
      roleIds: editUser.roleIds,
    }
    if (editUser.password) body.password = editUser.password
    await api('/api/v1/user/update', body)
    message.value = '用户已更新'
    editUser.password = ''
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function removeUser(user) {
  if (!window.confirm(`删除用户 ${user.username}？`)) return
  error.value = ''
  message.value = ''
  try {
    await api('/api/v1/user/delete', { id: user.id })
    if (editingId.value === user.id) editingId.value = 0
    message.value = '用户已删除'
    await load()
  } catch (err) {
    error.value = err.message
  }
}

function selectRole(role) {
  selectedRoleId.value = role.id
  editRole.name = role.name
  editRole.description = role.description || ''
  draftPerms.value = [...(role.permissions || [])]
}

function togglePerm(code) {
  if (selectedRole.value?.code === 'admin') return
  const next = new Set(draftPerms.value)
  if (next.has(code)) next.delete(code)
  else next.add(code)
  draftPerms.value = [...next]
}

async function addRole() {
  busy.value = 'role'
  error.value = ''
  message.value = ''
  try {
    const created = await api('/api/v1/role/create', { ...createRole, permissions: [] })
    createRole.code = ''
    createRole.name = ''
    createRole.description = ''
    message.value = '角色已创建，接着勾选权限'
    await load()
    const role = roles.value.find((item) => item.id === created.data.id)
    if (role) selectRole(role)
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function saveRole() {
  busy.value = 'save-role'
  error.value = ''
  message.value = ''
  try {
    await api('/api/v1/role/update', {
      id: selectedRoleId.value,
      name: editRole.name,
      description: editRole.description,
      permissions: draftPerms.value,
    })
    message.value = '角色已更新'
    await load()
    const role = roles.value.find((item) => item.id === selectedRoleId.value)
    if (role) selectRole(role)
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function removeRole(role) {
  if (!window.confirm(`删除角色 ${role.name}？`)) return
  error.value = ''
  try {
    await api('/api/v1/role/delete', { id: role.id })
    if (selectedRoleId.value === role.id) selectedRoleId.value = 0
    message.value = '角色已删除'
    await load()
  } catch (err) {
    error.value = err.message
  }
}

onMounted(load)
</script>

<template>
  <section class="page">
    <div class="page-head">
      <div>
        <h1>权限</h1>
        <p>用户通过角色获得权限。管理员角色固定拥有全部权限，并且至少保留一名启用的管理员。</p>
      </div>
    </div>
    <div class="tabs">
      <button v-if="showUsers" type="button" :class="{ on: tab === 'users' }" @click="tab = 'users'">用户</button>
      <button v-if="showRoles" type="button" :class="{ on: tab === 'roles' }" @click="tab = 'roles'">角色</button>
    </div>
    <p v-if="error" class="banner">{{ error }}</p>
    <p v-if="message" class="banner warn">{{ message }}</p>

    <template v-if="tab === 'users'">
      <article v-if="session.can('stock:user:create')" class="card form">
        <h2>新建用户</h2>
        <div class="form inline">
          <label>用户名<input v-model="createUser.username" placeholder="字母开头" /></label>
          <label>显示名<input v-model="createUser.displayName" placeholder="可留空" /></label>
          <label>密码<input v-model="createUser.password" type="password" /></label>
          <button class="primary" :disabled="busy === 'user'" @click="addUser">创建</button>
        </div>
        <div class="checks">
          <label v-for="role in roles" :key="role.id" class="check">
            <input v-model="createUser.roleIds" type="checkbox" :value="role.id" />
            {{ role.name }}
          </label>
        </div>
      </article>

      <div class="split">
        <article class="card">
          <h2>用户</h2>
          <table>
            <thead>
              <tr><th>账号</th><th>角色</th><th>状态</th><th></th></tr>
            </thead>
            <tbody>
              <tr v-for="user in users" :key="user.id">
                <td>{{ user.displayName }} <span class="muted">{{ user.username }}</span></td>
                <td>{{ roleNames(user) }}</td>
                <td>{{ user.enabled ? '启用' : '停用' }}</td>
                <td class="right">
                  <button v-if="session.can('stock:user:update')" class="ghost" @click="edit(user)">编辑</button>
                  <button v-if="session.can('stock:user:delete') && user.id !== session.user?.id" class="danger" @click="removeUser(user)">删除</button>
                </td>
              </tr>
            </tbody>
          </table>
        </article>
        <article v-if="editingUser && session.can('stock:user:update')" class="card form">
          <h2>编辑 {{ editingUser.username }}</h2>
          <label>显示名<input v-model="editUser.displayName" /></label>
          <label>新密码<input v-model="editUser.password" type="password" placeholder="留空则不修改" /></label>
          <label class="check">
            <input v-model="editUser.enabled" type="checkbox" :disabled="editingUser.id === session.user?.id" />
            启用
          </label>
          <div class="checks">
            <label v-for="role in roles" :key="role.id" class="check">
              <input v-model="editUser.roleIds" type="checkbox" :value="role.id" />
              {{ role.name }}
            </label>
          </div>
          <button class="primary" :disabled="busy === 'save-user'" @click="saveUser">保存</button>
        </article>
      </div>
    </template>

    <template v-else>
      <article v-if="session.can('stock:role:create')" class="card">
        <h2>新建角色</h2>
        <div class="form inline">
          <label>标识<input v-model="createRole.code" placeholder="例如 auditor" /></label>
          <label>名称<input v-model="createRole.name" /></label>
          <label>说明<input v-model="createRole.description" /></label>
          <button class="primary" :disabled="busy === 'role'" @click="addRole">创建</button>
        </div>
      </article>
      <div class="split">
        <article class="card">
          <h2>角色</h2>
          <button v-for="role in roles" :key="role.id" class="strategy" :class="{ on: role.id === selectedRoleId }" @click="selectRole(role)">
            <b>{{ role.name }} <span v-if="role.builtIn" class="muted">内置</span></b>
            <span class="muted">{{ role.code }} · {{ role.permissions.length }} 项权限</span>
          </button>
        </article>
        <article v-if="selectedRole" class="card form">
          <div class="spread">
            <h2>{{ selectedRole.name }}</h2>
            <button v-if="session.can('stock:role:delete') && !selectedRole.builtIn" class="danger" @click="removeRole(selectedRole)">删除</button>
          </div>
          <p v-if="selectedRole.code === 'admin'" class="note">管理员角色必须保留全部权限。</p>
          <label>名称<input v-model="editRole.name" :disabled="!session.can('stock:role:update')" /></label>
          <label>说明<input v-model="editRole.description" :disabled="!session.can('stock:role:update')" /></label>
          <div class="perm-groups">
            <section v-for="group in grouped" :key="group.name">
              <h3>{{ group.name }}</h3>
              <div class="checks">
                <label v-for="item in group.items" :key="item.code" class="check">
                  <input
                    type="checkbox"
                    :checked="draftPerms.includes(item.code)"
                    :disabled="selectedRole.code === 'admin' || !session.can('stock:role:update')"
                    @change="togglePerm(item.code)"
                  />
                  {{ item.name }}
                </label>
              </div>
            </section>
          </div>
          <button v-if="session.can('stock:role:update')" class="primary" :disabled="busy === 'save-role'" @click="saveRole">保存角色</button>
        </article>
      </div>
    </template>
  </section>
</template>
