import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const source = readFileSync(resolve("src/pages/KeysPage.tsx"), "utf8");

assert.equal(source.includes("confirm("), false, "KeysPage should not use native browser confirm dialogs");
assert.match(source, /role="alertdialog"/, "revocation confirmation should be an in-app alertdialog");
assert.match(source, /确认吊销/, "revocation dialog should keep an explicit destructive confirmation action");
