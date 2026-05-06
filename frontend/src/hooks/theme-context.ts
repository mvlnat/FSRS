import { createContext } from 'react';

export type ThemePreference = 'light' | 'dark' | 'system';
export type EffectiveTheme = 'light' | 'dark';

export interface ThemeContextType {
  theme: ThemePreference;
  effectiveTheme: EffectiveTheme;
  setTheme: (theme: ThemePreference) => void;
}

export const ThemeContext = createContext<ThemeContextType | null>(null);
