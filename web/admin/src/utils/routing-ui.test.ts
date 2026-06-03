import assert from "node:assert/strict";
import {
  addProviderToChain,
  moveProviderInChain,
  normalizeProviderRefs,
  providerLabel
} from "./routing-ui.js";

const providers = [
  { id: "prov-openai", name: "OpenAI 主线路" },
  { id: "prov-azure", name: "Azure 备用线路" },
  { id: "prov-openrouter", name: "OpenRouter 备用出口" }
];

assert.deepEqual(
  normalizeProviderRefs(["OpenAI 主线路", "prov-azure", "OpenAI Primary", "missing"], providers),
  ["prov-openai", "prov-azure", "prov-openai", "missing"]
);

assert.deepEqual(addProviderToChain(["prov-openai"], "prov-azure"), ["prov-openai", "prov-azure"]);
assert.deepEqual(addProviderToChain(["prov-openai"], "prov-openai"), ["prov-openai"]);

assert.deepEqual(moveProviderInChain(["prov-openai", "prov-azure", "prov-openrouter"], 2, -1), [
  "prov-openai",
  "prov-openrouter",
  "prov-azure"
]);
assert.deepEqual(moveProviderInChain(["prov-openai"], 0, -1), ["prov-openai"]);

assert.equal(providerLabel("prov-openai", providers), "OpenAI 主线路");
assert.equal(providerLabel("unknown-provider", providers), "unknown-provider");

console.log("routing-ui tests passed");
