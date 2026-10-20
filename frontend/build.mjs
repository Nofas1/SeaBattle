import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { build } from "esbuild";

const frontendRoot = resolve(fileURLToPath(new URL(".", import.meta.url)));

await mkdir(resolve(frontendRoot, "dist"), { recursive: true });
await build({
  entryPoints: [resolve(frontendRoot, "src", "app.ts")],
  outfile: resolve(frontendRoot, "dist", "bundle.js"),
  bundle: true,
  minify: true,
  sourcemap: false,
  target: ["es2022"],
  platform: "browser",
  tsconfigRaw: {
    compilerOptions: {
      strict: true,
      target: "ES2022",
      lib: ["ES2022", "DOM", "DOM.Iterable"],
    },
  },
});

console.log("Built the Sea Battle frontend.");
