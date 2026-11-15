<script setup>
import { computed, ref } from "vue";
import Heartbeat from "./Heartbeat.vue";
import ToolList from "./ToolList.vue";
import { about, stateOf } from "../systems.js";

const props = defineProps({ server: { type: Object, required: true } });

const open = ref(false);
const info = computed(() => about(props.server.name));
const state = computed(() => stateOf(props.server));
const panelId = computed(() => `tools-${props.server.name}`);

const lampLabel = computed(() => {
  const l = info.value.label;
  return { up: `${l} is up`, slow: `${l} is slow`, down: `${l} is down`, idle: `${l} not checked yet` }[state.value];
});
const hasLatency = computed(() => state.value === "up" || state.value === "slow");
</script>

<template>
  <li class="system">
    <button class="row" type="button" :aria-expanded="open" :aria-controls="panelId" @click="open = !open">
      <span class="lamp" :class="state" role="img" :aria-label="lampLabel"></span>
      <span class="who">
        <span class="name">{{ info.label }}</span>
        <span class="does">{{ info.does }}</span>
        <span v-if="server.error" class="error">{{ server.error }}</span>
      </span>
      <Heartbeat class="beats" :history="server.history" />
      <span class="metric latency">
        <template v-if="hasLatency">{{ server.latency_ms < 1 ? "<1" : server.latency_ms }} <small>ms</small></template>
        <small v-else>—</small>
      </span>
      <span class="metric count">
        {{ server.tools.length }} <small>{{ server.tools.length === 1 ? "tool" : "tools" }}</small>
      </span>
      <span class="chev" aria-hidden="true"></span>
    </button>
    <ToolList v-show="open" :id="panelId" :server="server" />
  </li>
</template>

<style scoped>
.system { border-bottom: 1px solid var(--rule); }

.row {
  display: grid;
  grid-template-columns: 1.25rem minmax(10rem, 1.1fr) minmax(0, 1.6fr) 5rem 5.5rem 1rem;
  align-items: center;
  gap: 1.25rem;
  width: 100%;
  padding: 1.25rem 0;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.row:hover .name { text-decoration: underline; text-underline-offset: 3px; }

.lamp {
  width: 0.75rem;
  height: 0.75rem;
  border-radius: 50%;
  background: var(--idle);
  justify-self: center;
}
.lamp.up { background: var(--up); }
.lamp.slow { background: var(--slow); }
.lamp.down {
  background: var(--down);
  animation: alarm 1.6s ease-out infinite;
}

@keyframes alarm {
  0% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--down) 55%, transparent); }
  100% { box-shadow: 0 0 0 0.6rem transparent; }
}

.who { min-width: 0; }
.name { display: block; font-weight: 600; font-size: 1.0625rem; }
.does { display: block; color: var(--ink-soft); font-size: var(--t-sm); }
.error { display: block; color: var(--down); font-size: var(--t-sm); overflow-wrap: anywhere; }

.metric {
  text-align: right;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.metric small { color: var(--ink-soft); font-size: var(--t-sm); }

.chev {
  width: 0.6rem;
  height: 0.6rem;
  border-right: 2px solid var(--ink-soft);
  border-bottom: 2px solid var(--ink-soft);
  transform: rotate(45deg);
  transition: transform 160ms ease;
  justify-self: center;
}
.row[aria-expanded="true"] .chev { transform: rotate(-135deg); }

@media (max-width: 760px) {
  .row {
    grid-template-columns: 1.25rem 1fr auto 1rem;
    grid-template-areas:
      "lamp who   latency chev"
      ".    beats beats   .";
    gap: 0.5rem 0.9rem;
  }
  .lamp { grid-area: lamp; }
  .who { grid-area: who; }
  .beats { grid-area: beats; justify-content: flex-start; height: 1.25rem; }
  .latency { grid-area: latency; }
  .count { display: none; }
  .chev { grid-area: chev; }
}

@media (prefers-reduced-motion: reduce) {
  .lamp.down { animation: none; }
  .chev { transition: none; }
}
</style>
