import assert from "node:assert/strict";
import {
  dateInputToExpiryISOString,
  expiryISOStringToDateInput,
  formatLocalKeyExpiryLabel
} from "./local-key-expiry.js";

assert.equal(dateInputToExpiryISOString("2026-05-26"), "2026-05-26T15:59:59.999Z");
assert.equal(expiryISOStringToDateInput("2026-05-26T15:59:59.999Z"), "2026-05-26");
assert.equal(expiryISOStringToDateInput("2026-05-25T23:59:59Z"), "2026-05-26");
assert.equal(formatLocalKeyExpiryLabel("2026-05-26T15:59:59.999Z"), "到期 2026-05-26");
assert.equal(formatLocalKeyExpiryLabel(null), "长期有效");
