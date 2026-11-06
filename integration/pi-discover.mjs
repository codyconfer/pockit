// Exercise the installed Pi package loader and real extension lifecycle, without
// a model request or API key. Do not inject extra extension/skill paths: discovery
// must come from the settings written by `pockit ... --pi-extension --install`.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";

const { DefaultResourceLoader, SessionManager, createAgentSession } = await import(
  pathToFileURL(process.env.PI_MODULE_PATH).href
);
const cwd = process.cwd();
const agentDir = join(process.env.HOME, ".pi", "agent");
const packageRoot = resolve(process.argv[2]);
const loader = new DefaultResourceLoader({ cwd, agentDir });
await loader.reload();
const loaded = loader.getExtensions();
assert.deepEqual(loaded.errors, [], "Pi extension load errors");
assert.equal(loaded.extensions.length, 1, "installed extension not discovered");
assert.equal(resolve(loaded.extensions[0].path), join(packageRoot, "index.ts"));

const errors = [];
const { session } = await createAgentSession({
  cwd, agentDir, resourceLoader: loader,
  sessionManager: SessionManager.inMemory(), tools: [],
});
try {
  await session.bindExtensions({ onError: (error) => errors.push(error) });
  assert.deepEqual(errors, [], "Pi extension lifecycle errors");
  const { skills, diagnostics } = loader.getSkills();
  assert.deepEqual(diagnostics, [], "Pi skill diagnostics");
  assert.deepEqual(skills.map((skill) => skill.name).sort(), ["fixture-base", "fixture-child"]);
  for (const skill of skills) {
    assert.equal(resolve(skill.filePath), join(packageRoot, skill.name, "SKILL.md"));
    assert.match(readFileSync(skill.filePath, "utf8"), /bundle-index\.json/);
  }
  console.log("Pi discovered both installed skills through resources_discover");
} finally {
  session.dispose();
}
