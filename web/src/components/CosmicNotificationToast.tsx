import React, { useEffect } from 'react';
import { Info, X } from 'lucide-react';

interface CosmicNotificationToastProps {
  notification: { title: string; message: string; id: number } | null;
  onClose: () => void;
}

export const CosmicNotificationToast: React.FC<CosmicNotificationToastProps> = ({ notification, onClose }) => {
  useEffect(() => {
    if (notification) {
      const timer = setTimeout(() => {
        onClose();
      }, 4500);
      return () => clearTimeout(timer);
    }
  }, [notification, onClose]);

  if (!notification) return null;

  return (
    <div className="fixed top-4 right-4 z-50 flex items-start max-w-sm w-full bg-slate-950/90 border border-cyan-500/40 shadow-lg shadow-cyan-500/10 backdrop-blur-md rounded-lg p-3 text-cyan-200 pointer-events-auto">
      <div className="shrink-0 mt-0.5">
        <Info className="w-5 h-5 text-cyan-400" />
      </div>
      <div className="ml-3 flex-1">
        <h3 className="text-sm font-semibold text-cyan-100">{notification.title}</h3>
        <p className="mt-1 text-sm text-cyan-200/80">{notification.message}</p>
      </div>
      <button 
        onClick={onClose} 
        className="ml-4 shrink-0 text-cyan-500 hover:text-cyan-300 transition-colors"
        aria-label="Close notification"
      >
        <X className="w-4 h-4" />
      </button>
    </div>
  );
};
