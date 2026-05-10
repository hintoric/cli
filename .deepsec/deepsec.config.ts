import { defineConfig } from "deepsec/config";

export default defineConfig({
  projects: [
    { id: "cli", root: ".." },
    // <deepsec:projects-insert-above>
  ],
});
