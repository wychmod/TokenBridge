export function providerNeedsAlert(status: string): boolean {
  const normalized = status.trim().toLowerCase();
  if (!normalized) return false;
  return !["healthy", "active", "disabled", "deleted"].includes(normalized);
}
