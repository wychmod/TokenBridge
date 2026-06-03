import { labelFromMap, providerNameLabelMap } from "../store/labels.js";

export type ProviderOptionLike = {
  id: string;
  name: string;
};

export function normalizeProviderRefs(refs: string[], providers: ProviderOptionLike[]): string[] {
  return refs.map((ref) => providerValue(ref, providers));
}

export function providerValue(ref: string, providers: ProviderOptionLike[]): string {
  const trimmed = ref.trim();
  if (!trimmed) return "";
  const direct = providers.find((provider) => same(provider.id, trimmed) || same(provider.name, trimmed));
  if (direct) return direct.id;

  const localized = labelFromMap(providerNameLabelMap, trimmed);
  const byLocalizedName = providers.find((provider) => same(provider.name, localized));
  if (byLocalizedName) return byLocalizedName.id;

  return trimmed;
}

export function providerLabel(ref: string, providers: ProviderOptionLike[]): string {
  const normalized = providerValue(ref, providers);
  const provider = providers.find((item) => same(item.id, normalized) || same(item.name, normalized));
  if (provider) return provider.name;
  return labelFromMap(providerNameLabelMap, ref);
}

export function addProviderToChain(chain: string[], providerId: string): string[] {
  const value = providerId.trim();
  if (!value || chain.includes(value)) return chain;
  return [...chain, value];
}

export function removeProviderFromChain(chain: string[], index: number): string[] {
  if (index < 0 || index >= chain.length) return chain;
  return chain.filter((_, currentIndex) => currentIndex !== index);
}

export function moveProviderInChain(chain: string[], index: number, direction: -1 | 1): string[] {
  const nextIndex = index + direction;
  if (index < 0 || index >= chain.length || nextIndex < 0 || nextIndex >= chain.length) {
    return chain;
  }
  const next = [...chain];
  const [item] = next.splice(index, 1);
  next.splice(nextIndex, 0, item);
  return next;
}

function same(left: string, right: string): boolean {
  return left.trim().toLowerCase() === right.trim().toLowerCase();
}
