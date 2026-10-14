export function num(value, digits = 2) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString('zh-CN', { minimumFractionDigits: digits, maximumFractionDigits: digits })
}

export function pct(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  return `${n > 0 ? '+' : ''}${n.toFixed(2)}%`
}

export function signed(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  return `${n > 0 ? '+' : ''}${num(n)}`
}

export function cls(value) {
  const n = Number(value)
  if (!Number.isFinite(n) || n === 0) return 'flat'
  return n > 0 ? 'up' : 'down'
}

export function compact(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  const abs = Math.abs(n)
  if (abs >= 1e8) return `${(n / 1e8).toFixed(2)}亿`
  if (abs >= 1e4) return `${(n / 1e4).toFixed(2)}万`
  return num(n, 0)
}

export function when(value) {
  if (!value) return '—'
  if (typeof value === 'string' && /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}/.test(value)) {
    return value.slice(5, 16)
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
    timeZone: 'Asia/Shanghai',
  }).format(date)
}

export function marketLabel(market) {
  return { sh: '沪', sz: '深', bj: '京' }[market] || ''
}

export function dirLabel(direction) {
  return { up: '看涨', down: '看跌', flat: '震荡' }[direction] || '—'
}

export function sideLabel(side) {
  return side === 'buy' ? '买入' : side === 'sell' ? '卖出' : side || '—'
}

export function kindLabel(kind) {
  return {
    price_above: '价格高于',
    price_below: '价格低于',
    pct_above: '涨幅达到',
    pct_below: '跌幅达到',
    ma_cross: '双均线',
    rsi: 'RSI 回归',
    momentum: '动量突破',
  }[kind] || kind
}

export function signalLabel(signal) {
  return signal === 'long' ? '持有' : signal === 'flat' ? '空仓' : '—'
}
