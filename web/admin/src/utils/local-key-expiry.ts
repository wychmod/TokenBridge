const BEIJING_OFFSET_MS = 8 * 60 * 60 * 1000;
const DATE_INPUT_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/;

export function dateInputToExpiryISOString(dateInput: string): string | null {
  const match = dateInput.trim().match(DATE_INPUT_PATTERN);
  if (!match) return null;

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  if (!isValidDateParts(year, month, day)) return null;

  return new Date(Date.UTC(year, month - 1, day, 15, 59, 59, 999)).toISOString();
}

export function expiryISOStringToDateInput(expiresAt?: string | null): string {
  if (!expiresAt) return "";
  const date = new Date(expiresAt);
  if (Number.isNaN(date.getTime())) return "";

  const beijingDate = new Date(date.getTime() + BEIJING_OFFSET_MS);
  return [
    beijingDate.getUTCFullYear(),
    pad2(beijingDate.getUTCMonth() + 1),
    pad2(beijingDate.getUTCDate())
  ].join("-");
}

export function formatLocalKeyExpiryLabel(expiresAt?: string | null): string {
  const date = expiryISOStringToDateInput(expiresAt);
  return date ? `到期 ${date}` : "长期有效";
}

function isValidDateParts(year: number, month: number, day: number): boolean {
  const date = new Date(Date.UTC(year, month - 1, day));
  return date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day;
}

function pad2(value: number): string {
  return value.toString().padStart(2, "0");
}
