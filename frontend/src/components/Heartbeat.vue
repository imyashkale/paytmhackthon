<script setup>
import { computed } from "vue";
import { HISTORY, SLOW_MS } from "../systems.js";

const props = defineProps({ history: { type: Array, required: true } });

// One tick per check, padded on the left so the newest is always on the right.
const ticks = computed(() => {
  const pad = Array.from({ length: Math.max(0, HISTORY - props.history.length) }, () => ({ kind: "", h: 30 }));
  const max = Math.max(SLOW_MS, ...props.history.map((c) => c.latency_ms));
  const seen = props.history.map((c) => {
    if (!c.ok) return { kind: "fail", h: 100 };
    // Log scale so a 2 ms and a 20 ms check are both visible.
    const h = 30 + 70 * (Math.log1p(c.latency_ms) / Math.log1p(max));
    return { kind: c.latency_ms > SLOW_MS ? "slow" : "ok", h: Math.round(h) };
  });
  return [...pad, ...seen];
});
</script>

<template>
  <span class="beats" aria-hidden="true">
    <i v-for="(t, i) in ticks" :key="i" class="tick" :class="t.kind" :style="{ height: `${t.h}%` }"></i>
  </span>
</template>

<style scoped>
.beats {
  display: flex;
  align-items: flex-end;
  justify-content: flex-end;
  gap: 2px;
  height: 1.75rem;
  min-width: 0;
  overflow: hidden;
}
</style>
