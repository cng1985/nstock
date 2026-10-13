<script setup>
import * as echarts from 'echarts'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps({
  curve: { type: Array, default: () => [] },
})
const el = ref(null)
let chart
let observer

function render() {
  if (!chart) return
  const up = (props.curve.at(-1)?.equity || 0) >= (props.curve[0]?.equity || 0)
  chart.setOption({
    backgroundColor: 'transparent',
    animation: false,
    grid: { left: 64, right: 16, top: 20, bottom: 28 },
    tooltip: { trigger: 'axis' },
    xAxis: {
      type: 'category',
      data: props.curve.map((point) => point.date),
      axisLabel: { color: '#93a1b0' },
      axisLine: { lineStyle: { color: '#31404e' } },
    },
    yAxis: {
      scale: true,
      axisLabel: { color: '#93a1b0' },
      splitLine: { lineStyle: { color: '#26323d' } },
    },
    series: [{
      type: 'line',
      showSymbol: false,
      data: props.curve.map((point) => point.equity),
      lineStyle: { width: 2, color: up ? '#ff4d5e' : '#1ec98a' },
      areaStyle: { color: up ? 'rgba(255,77,94,0.14)' : 'rgba(30,201,138,0.12)' },
    }],
  }, true)
}

onMounted(() => {
  chart = echarts.init(el.value)
  render()
  observer = new ResizeObserver(() => chart?.resize())
  observer.observe(el.value)
})
watch(() => props.curve, render, { deep: true })
onBeforeUnmount(() => {
  observer?.disconnect()
  chart?.dispose()
})
</script>

<template>
  <div ref="el" class="chart short"></div>
</template>
