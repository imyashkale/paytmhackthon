<script setup>
import { computed } from "vue";
import { plural } from "../systems.js";

const props = defineProps({ server: { type: Object, required: true } });

const intro = computed(() => {
  const s = props.server;
  if (s.tools.length === 0) {
    return s.healthy ? "This system has no tools." : "No tools seen yet. They'll appear once this system responds.";
  }
  if (!s.healthy) return "These tools are unavailable until the system responds again.";
  return `Your agent can call these ${plural(s.tools.length, "tool", "tools")}${s.version ? ` (server version ${s.version})` : ""}.`;
});
</script>

<template>
  <div class="tools">
    <p class="intro">{{ intro }}</p>
    <article v-for="t in server.tools" :key="t.name" class="tool">
      <h3>{{ t.name }}</h3>
      <p v-if="t.description">{{ t.description }}</p>
      <dl v-if="t.params.length" class="params">
        <template v-for="p in t.params" :key="p.name">
          <dt>{{ p.name }}</dt>
          <span class="type">{{ p.type }} <span v-if="p.required" class="req">required</span></span>
          <dd>{{ p.description }}</dd>
        </template>
      </dl>
      <p v-else class="no-params">Takes no inputs.</p>
    </article>
  </div>
</template>

<style scoped>
.tools { margin: 0 0 1.5rem 2.5rem; }

.intro {
  margin: 0.25rem 0 1rem;
  color: var(--ink-soft);
  font-size: var(--t-sm);
}

.tool {
  padding: 1rem 1.25rem;
  background: var(--surface);
  border-left: 3px solid var(--ink);
}
.tool + .tool { margin-top: 2px; }

h3 {
  margin: 0;
  font: 500 var(--t-base)/1.4 var(--mono);
}
.tool > p {
  margin: 0.25rem 0 0;
  max-width: 68ch;
  color: var(--ink-soft);
}

.params {
  display: grid;
  grid-template-columns: max-content max-content 1fr;
  gap: 0.35rem 1.25rem;
  margin: 0.85rem 0 0;
  font-size: var(--t-sm);
}
dt { font-family: var(--mono); font-weight: 500; }
.type { font-family: var(--mono); color: var(--ink-soft); }
dd { margin: 0; color: var(--ink-soft); max-width: 60ch; }
.req { color: var(--ink); font-family: var(--sans); font-weight: 600; }
.tool > .no-params { margin-top: 0.6rem; font-size: var(--t-sm); }

@media (max-width: 760px) {
  .tools { margin-left: 0; }
  .params { grid-template-columns: max-content 1fr; }
  dd { grid-column: 1 / -1; margin-bottom: 0.4rem; }
}
</style>
