import { createContext } from 'react';

export interface DemoContextType {
  isDemo: boolean;
  enterDemo: () => void;
  exitDemo: () => void;
  promptSignup: (action: string) => void;
}

export const DemoContext = createContext<DemoContextType | null>(null);
