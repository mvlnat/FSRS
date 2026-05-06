import { useState, useCallback, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { DemoContext } from './demo-context';
import { enableDemoMode, disableDemoMode } from '../demo/demoApi';

export function DemoProvider({ children }: { children: ReactNode }) {
  const [isDemo, setIsDemo] = useState(false);
  const [showPrompt, setShowPrompt] = useState(false);
  const [promptAction, setPromptAction] = useState('');
  const navigate = useNavigate();

  const enterDemo = useCallback(() => {
    enableDemoMode();
    setIsDemo(true);
  }, []);

  const exitDemo = useCallback(() => {
    disableDemoMode();
    setIsDemo(false);
    setShowPrompt(false);
  }, []);

  const promptSignup = useCallback((action: string) => {
    setPromptAction(action);
    setShowPrompt(true);
  }, []);

  const handleSignup = () => {
    exitDemo();
    navigate('/register');
  };

  const handleContinue = () => {
    setShowPrompt(false);
  };

  return (
    <DemoContext.Provider value={{ isDemo, enterDemo, exitDemo, promptSignup }}>
      {children}
      {showPrompt && (
        <div className="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="signup-prompt-title">
          <div className="modal signup-prompt-modal">
            <h2 id="signup-prompt-title">Create an Account</h2>
            <p>
              {promptAction
                ? `To ${promptAction}, you'll need to create a free account.`
                : 'Create a free account to save your progress and access your flashcards from any device.'}
            </p>
            <p className="signup-prompt-benefit">
              Your study progress and custom decks will be saved securely.
            </p>
            <div className="signup-prompt-actions">
              <button onClick={handleSignup} className="btn-primary">
                Sign Up Free
              </button>
              <button onClick={handleContinue} className="btn-secondary">
                Continue Demo
              </button>
            </div>
          </div>
        </div>
      )}
      {isDemo && (
        <div className="demo-banner">
          <span>Demo Mode</span>
          <button onClick={() => promptSignup('')} className="demo-banner-signup">
            Sign up to save
          </button>
        </div>
      )}
    </DemoContext.Provider>
  );
}
