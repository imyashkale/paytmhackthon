<script setup>
import { computed, onMounted, onUnmounted, ref } from "vue";
import SystemRow from "./components/SystemRow.vue";
import { about, joinNames, neverChecked, plural, stateOf } from "./systems.js";

const servers = ref([]);
const lastChecked = ref(null);
const intervalMs = ref(5000);
const fetchFailed = ref(false);
const now = ref(Date.now());

let pollTimer;
let clockTimer;

async function poll() {
  try {
    const res = await fetch("api/status", { cache: "no-store" });
    if (!res.ok) throw new Error(`status ${res.status}`);
    const data = await res.json();
    servers.value = data.servers;
    intervalMs.value = data.interval_ms || intervalMs.value;
    if (!neverChecked(data.checked_at)) lastChecked.value = Date.parse(data.checked_at);
    fetchFailed.value = false;
  } catch {
    fetchFailed.value = true;
  }
  pollTimer = setTimeout(poll, intervalMs.value);
}

onMounted(() => {
  poll();
  clockTimer = setInterval(() => (now.value = Date.now()), 1000);
});
onUnmounted(() => {
  clearTimeout(pollTimer);
  clearInterval(clockTimer);
});

const checked = computed(() => servers.value.some((s) => stateOf(s) !== "idle"));
const down = computed(() => servers.value.filter((s) => stateOf(s) === "down"));
const totalTools = computed(() => servers.value.reduce((n, s) => n + s.tools.length, 0));
const liveTools = computed(() => servers.value.filter((s) => s.healthy).reduce((n, s) => n + s.tools.length, 0));

const unreachable = computed(() => fetchFailed.value && servers.value.length === 0);

const headline = computed(() => {
  if (unreachable.value) return "Can't reach the monitor.";
  if (!checked.value) return "Checking your systems…";
  const n = servers.value.length;
  const d = down.value.length;
  if (d === 0) return `All ${n} systems are up.`;
  return `${d} of ${n} systems ${d === 1 ? "is" : "are"} down.`;
});

const subline = computed(() => {
  if (unreachable.value) {
    return "This page gets its data from the monitor backend at /api/status. Check that the monitor is running. The page retries on its own.";
  }
  if (!checked.value) return "This page checks every MCP server a few times a minute.";
  const d = down.value;
  if (d.length === 0) {
    return `Your agent can use all ${plural(totalTools.value, "tool", "tools")} across ${joinNames(servers.value.map((s) => about(s.name).label))}.`;
  }
  const names = joinNames(d.map((s) => about(s.name).label));
  const one = d.length === 1;
  const still = liveTools.value > 0 ? ` The other ${plural(liveTools.value, "tool still works", "tools still work")}.` : "";
  return `${names} ${one ? "isn't" : "aren't"} responding, so your agent can't use ${one ? "its" : "their"} tools right now.${still} Open a system below to see the error.`;
});

const checkedText = computed(() => {
  if (fetchFailed.value) return "Can't reach the status service. Retrying…";
  if (!lastChecked.value) return "Connecting…";
  const secs = Math.max(0, Math.round((now.value - lastChecked.value) / 1000));
  return secs < 2 ? "Checked just now" : `Checked ${secs}s ago`;
});

const stale = computed(
  () => fetchFailed.value || (lastChecked.value && now.value - lastChecked.value > intervalMs.value * 3),
);
</script>

<template>
  <header class="bar">
    <span class="wordmark">AI Teammates</span>
    <span class="checked" :class="{ stale }">{{ checkedText }}</span>
  </header>

  <main>
    <section class="summary" aria-live="polite">
      <h1 :class="{ 'is-down': down.length > 0 || unreachable }">{{ headline }}</h1>
      <p>{{ subline }}</p>
    </section>

    <section aria-labelledby="systems-title">
      <div class="list-head">
        <h2 id="systems-title">Systems</h2>
        <p class="legend">
          <span><i class="tick ok"></i>Responded</span>
          <span><i class="tick fail"></i>No response</span>
          <span>Last 5 minutes, newest on the right</span>
        </p>
      </div>
      <ul class="systems">
        <SystemRow v-for="s in servers" :key="s.name" :server="s" />
      </ul>
    </section>
  </main>
</template>

<style scoped>
.bar {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 1rem;
  max-width: 72rem;
  margin: 0 auto;
  padding: 1.25rem var(--gutter);
}

.wordmark {
  font-weight: 800;
  letter-spacing: -0.01em;
}

.checked {
  color: var(--ink-soft);
  font-size: var(--t-sm);
  font-variant-numeric: tabular-nums;
}
.checked.stale { color: var(--down); }

main {
  max-width: 72rem;
  margin: 0 auto;
  padding: 0 var(--gutter) 4rem;
}

/* The one loud element on the page. */
.summary {
  max-width: 52rem;
  padding: clamp(2rem, 7vw, 5.5rem) 0 clamp(2rem, 5vw, 3.5rem);
}

.summary h1 {
  margin: 0;
  font-size: var(--t-hero);
  font-weight: 800;
  line-height: 1.02;
  letter-spacing: -0.035em;
  text-wrap: balance;
}
.summary h1.is-down { color: var(--down); }

.summary p {
  margin: 1rem 0 0;
  max-width: 40rem;
  font-size: var(--t-md);
  line-height: 1.4;
  color: var(--ink-soft);
  text-wrap: pretty;
}

.list-head {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: baseline;
  gap: 0.5rem 2rem;
  padding-bottom: 0.75rem;
  border-bottom: 2px solid var(--ink);
}

.list-head h2 {
  margin: 0;
  font-size: var(--t-md);
  font-weight: 600;
}

.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem 1.25rem;
  margin: 0;
  font-size: var(--t-sm);
  color: var(--ink-soft);
}
.legend span { display: inline-flex; align-items: center; gap: 0.4rem; }
.legend .tick { height: 12px; }

.systems {
  list-style: none;
  margin: 0;
  padding: 0;
}
</style>
