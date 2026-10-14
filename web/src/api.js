export async function api(path, body) {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json;charset=UTF-8' },
    body: JSON.stringify(body ?? {}),
  })
  const payload = await res.json().catch(() => ({}))
  if (!res.ok || payload.code !== 200) {
    throw new Error(payload.msg || '请求失败')
  }
  return payload
}
