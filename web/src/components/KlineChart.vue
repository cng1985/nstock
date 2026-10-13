<script setup>
import * as echarts from 'echarts'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps({
  bars: { type: Array, default: () => [] },
})

const el = ref(null)
let chart
let observer

function ma(period) {
  return props.bars.map((_, index) => {
    if (index + 1 < period) return null
    let sum = 0
    for (let i = index + 1 - period; i <= index; i += 1) sum += props.bars[i].close
    return Number((sum / period).toFixed(2))
  })
}

function render() {
  if (!chart) return
  const dates = props.bars.map((bar) => bar.date)
  const candles = props.bars.map((bar) => [bar.open, bar.close, bar.low, bar.high])
  const volumes = props.bars.map((bar) => ({
    value: bar.volume,
    itemStyle: { color: bar.close >= bar.open ? '#ff4d5e' : '#1ec98a' },
  }))
  chart.setOption({
    backgroundColor: 'transparent',
    animation: false,
    legend: { data: ['K线', 'MA5', 'MA20'], textStyle: { color: '#93a1b0' }, top: 0 },
    tooltip: { trigger: 'axis', axisPointer: { type: 'cross' } },
    axisPointer: { link: [{ xAxisIndex: [0, 1] }] },
    grid: [
      { left: 64, right: 16, top: 36, height: '58%' },
      { left: 64, right: 16, top: '76%', height: '14%' },
    ],
    xAxis: [
      { type: 'category', data: dates, boundaryGap: true, axisLine: { lineStyle: { color: '#31404e' } }, axisLabel: { color: '#93a1b0' } },
      { type: 'category', gridIndex: 1, data: dates, axisLabel: { show: false }, axisLine: { lineStyle: { color: '#31404e' } } },
    ],
    yAxis: [
      { scale: true, axisLabel: { color: '#93a1b0' }, splitLine: { lineStyle: { color: '#26323d' } } },
      { gridIndex: 1, axisLabel: { show: false }, splitLine: { show: false } },
    ],
    dataZoom: [
      { type: 'inside', xAxisIndex: [0, 1], start: 55, end: 100 },
      { type: 'slider', xAxisIndex: [0, 1], height: 16, bottom: 4, borderColor: '#31404e', fillerColor: 'rgba(226,193,122,0.15)' },
    ],
    series: [
      { name: 'K线', type: 'candlestick', data: candles, itemStyle: { color: '#ff4d5e', color0: '#1ec98a', borderColor: '#ff4d5e', borderColor0: '#1ec98a' } },
      { name: 'MA5', type: 'line', data: ma(5), showSymbol: false, lineStyle: { width: 1.2, color: '#e2c17a' } },
      { name: 'MA20', type: 'line', data: ma(20), showSymbol: false, lineStyle: { width: 1.2, color: '#9ec0ff' } },
      { name: '成交量', type: 'bar', xAxisIndex: 1, yAxisIndex: 1, data: volumes },
    ],
  }, true)
}

onMounted(() => {
  chart = echarts.init(el.value)
  render()
  observer = new ResizeObserver(() => chart?.resize())
  observer.observe(el.value)
})
watch(() => props.bars, render, { deep: true })
onBeforeUnmount(() => {
  observer?.disconnect()
  chart?.dispose()
})
</script>

<template>
  <div ref="el" class="chart"></div>
</template>
