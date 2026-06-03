import assert from "node:assert/strict";
import { providerNeedsAlert } from "./dashboard-alerts.js";

assert.equal(providerNeedsAlert("healthy"), false);
assert.equal(providerNeedsAlert("active"), false);
assert.equal(providerNeedsAlert("disabled"), false);
assert.equal(providerNeedsAlert("warning"), true);
assert.equal(providerNeedsAlert("blocked"), true);
assert.equal(providerNeedsAlert("unavailable"), true);

console.log("dashboard-alerts tests passed");
