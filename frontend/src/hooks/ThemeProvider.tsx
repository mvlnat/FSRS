import { useState, useEffect, useCallback, useMemo, type ReactNode } from 'react';
import { ThemeContext, type ThemePreference, type EffectiveTheme } from './theme-context';

const STORAGE_KEY = 'fsrs-theme';

function getSystemTheme(): EffectiveTheme {
  if (typeof window === 'undefined' || !window.matchMedia) {
    return 'light';
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function getStoredTheme(): ThemePreference {
  if (typeof window === 'undefined') {
    return 'system';
  }
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === 'light' || stored === 'dark' || stored === 'system') {
    return stored;
  }
  return 'system';
}

function applyThemeToDOM(effective: EffectiveTheme) {
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-theme', effective);
  }
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ThemePreference>(getStoredTheme);
  const [systemTheme, setSystemTheme] = useState<EffectiveTheme>(getSystemTheme);

  const effectiveTheme = useMemo((): EffectiveTheme => {
    if (theme === 'system') {
      return systemTheme;
    }
    return theme;
  }, [theme, systemTheme]);

  // Apply theme to DOM whenever effectiveTheme changes
  useEffect(() => {
    applyThemeToDOM(effectiveTheme);
  }, [effectiveTheme]);

  // Listen for system theme changes
  useEffect(() => {
    if (typeof window === 'undefined' || !window.matchMedia) {
      return;
    }

    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    const handleChange = (e: MediaQueryListEvent) => {
      setSystemTheme(e.matches ? 'dark' : 'light');
    };

    mediaQuery.addEventListener('change', handleChange);
    return () => mediaQuery.removeEventListener('change', handleChange);
  }, []);

  const setTheme = useCallback((newTheme: ThemePreference) => {
    localStorage.setItem(STORAGE_KEY, newTheme);
    setThemeState(newTheme);
  }, []);

  return (
    <ThemeContext.Provider value={{ theme, effectiveTheme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}
