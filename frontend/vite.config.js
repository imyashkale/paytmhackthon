import { spawn } from "node:child_process";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

// In dev, /api is proxied to the monitor backend. By default the dev server
// port-forwards the in-cluster mcp-monitor Service to localhost:MONITOR_PORT.
// Set MONITOR_URL to use a monitor running somewhere else instead.
const port = process.env.MONITOR_PORT || "8081";
const target = process.env.MONITOR_URL || `http://localhost:${port}`;

function monitorTunnel() {
  return {
    name: "monitor-tunnel",
    apply: "serve",
    configureServer(server) {
      if (process.env.MONITOR_URL) return;
      const log = server.config.logger;
      const pf = spawn("kubectl", ["-n", "mcp", "port-forward", "svc/mcp-monitor", `${port}:80`], {
        stdio: ["ignore", "ignore", "pipe"],
      });
      log.info(`[monitor] forwarding mcp-monitor to ${target}`);
      pf.stderr.on("data", (d) => log.error(`[monitor] ${d.toString().trim()}`));
      pf.on("error", (e) =>
        log.error(`[monitor] couldn't run kubectl (${e.message}). Set MONITOR_URL to your monitor's address.`),
      );
      pf.on("exit", (code) => {
        if (code) log.error(`[monitor] port-forward stopped (exit ${code}). Is the cluster up? Try 'make k8s-deploy'.`);
      });
      const stop = () => pf.exitCode === null && pf.kill();
      server.httpServer?.on("close", stop);
      process.on("exit", stop);
      for (const sig of ["SIGINT", "SIGTERM", "SIGHUP"]) {
        process.once(sig, () => {
          stop();
          process.exit(130);
        });
      }
    },
  };
}

export default defineConfig({
  plugins: [vue(), monitorTunnel()],
  base: "./",
  build: { outDir: "dist", emptyOutDir: true },
  server: {
    proxy: { "/api": target },
  },
});
