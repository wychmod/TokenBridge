import { create } from "zustand";

type ThemeMode = "light" | "dark" | "system";

type UIState = {
  theme: ThemeMode;
  initialized: boolean;
  setTheme: (theme: ThemeMode) => void;
  setInitialized: (initialized: boolean) => void;
};

export const useUIStore = create<UIState>((set) => ({
  theme: "system",
  initialized: true,
  setTheme: (theme) => set({ theme }),
  setInitialized: (initialized) => set({ initialized })
}));
