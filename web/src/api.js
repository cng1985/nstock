const TOKEN_KEY = 'nstock.token'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export async function api(path, body) {
  const headers = { 'Content-Type': 'application/json;charset=UTF-8' }
  const token = getToken()
  if (token) headers.Authorization = `Bearer ${token}`
  const res = await fetch(path, {
    method: 'POST',
    headers,
    body: JSON.stringify(body ?? {}),
  })
  const payload = await res.json().catch(() => ({}))
  if (res.status === 401 && !String(path).endsWith('/auth/login')) {
    setToken('')
    if (!window.location.pathname.startsWith('/login')) {
      const next = window.location.pathname + window.location.search
      window.location.assign(`/login?redirect=${encodeURIComponent(next)}`)
    }
  }
  if (!res.ok || payload.code !== 200) {
    throw new Error(payload.msg || '请求失败')
  }
  return payload
}
