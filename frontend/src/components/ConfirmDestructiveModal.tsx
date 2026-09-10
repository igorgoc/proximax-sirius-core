import React, { useState, useEffect } from 'react';
import { AlertTriangle, ShieldAlert, X } from 'lucide-react';

interface ConfirmDestructiveModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  description: string;
  expectedText: string;
  confirmButtonText?: string;
  inProgress?: boolean;
}

export const ConfirmDestructiveModal: React.FC<ConfirmDestructiveModalProps> = ({
  isOpen,
  onClose,
  onConfirm,
  title,
  description,
  expectedText,
  confirmButtonText = 'Confirm Destructive Action',
  inProgress = false,
}) => {
  const [typedText, setTypedText] = useState('');

  useEffect(() => {
    if (isOpen) {
      setTypedText('');
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const isMatch = typedText.trim().toUpperCase() === expectedText.trim().toUpperCase();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (isMatch && !inProgress) {
      onConfirm();
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-xs animate-in fade-in duration-150">
      <div className="w-full max-w-md bg-[#13171F] border border-rose-900/60 rounded-xl shadow-2xl overflow-hidden animate-in zoom-in-95 duration-150">
        
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-rose-900/40 bg-rose-950/20">
          <div className="flex items-center space-x-2.5">
            <div className="w-8 h-8 rounded-lg bg-rose-950/60 border border-rose-700/50 flex items-center justify-center text-rose-400 flex-shrink-0">
              <ShieldAlert className="w-4 h-4" />
            </div>
            <div>
              <h3 className="text-sm font-bold text-white tracking-wide">{title}</h3>
              <span className="text-[10px] font-mono text-rose-400 font-semibold uppercase">
                Destructive Operation
              </span>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            disabled={inProgress}
            className="p-1 text-slate-400 hover:text-white rounded-lg transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Content Body */}
        <form onSubmit={handleSubmit} className="p-5 space-y-4">
          <div className="p-3 bg-rose-950/20 border border-rose-900/40 rounded-lg space-y-2">
            <div className="flex items-start space-x-2 text-rose-300 text-xs leading-relaxed">
              <AlertTriangle className="w-4 h-4 text-rose-400 flex-shrink-0 mt-0.5" />
              <span>{description}</span>
            </div>
            <p className="text-[11px] text-rose-400/80 font-mono">
              The node engine will be paused and existing chain data will be overwritten or deleted. This operation cannot be undone.
            </p>
          </div>

          <div className="space-y-1.5">
            <label className="text-xs text-slate-300 block">
              To confirm, type <strong className="text-white font-mono bg-zinc-800 px-1.5 py-0.5 rounded border border-zinc-700">{expectedText}</strong> below:
            </label>
            <input
              type="text"
              autoFocus
              value={typedText}
              onChange={(e) => setTypedText(e.target.value)}
              placeholder={`Type "${expectedText}" to confirm`}
              disabled={inProgress}
              className="w-full px-3 py-2 bg-[#0F1115] border border-[#262B34] focus:border-rose-500 rounded-lg text-xs font-mono text-white placeholder-slate-600 focus:outline-none transition-all uppercase"
            />
          </div>

          {/* Action Buttons */}
          <div className="pt-2 flex items-center justify-end space-x-3">
            <button
              type="button"
              onClick={onClose}
              disabled={inProgress}
              className="px-4 py-2 bg-[#181B20] hover:bg-[#262B34] text-slate-300 hover:text-white rounded-lg text-xs font-semibold border border-[#262B34] transition-all"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!isMatch || inProgress}
              className="px-4 py-2 bg-rose-600 hover:bg-rose-500 disabled:opacity-30 disabled:cursor-not-allowed text-white rounded-lg text-xs font-semibold shadow-xs transition-all flex items-center space-x-1.5"
            >
              <ShieldAlert className="w-3.5 h-3.5" />
              <span>{inProgress ? 'Executing...' : confirmButtonText}</span>
            </button>
          </div>
        </form>

      </div>
    </div>
  );
};
