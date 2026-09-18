import React from 'react';
import { ExternalLink } from 'lucide-react';
import { SiriusLogo } from './SiriusLogo';

interface AboutModalProps {
  isOpen: boolean;
  onClose: () => void;
  version?: string;
}

export const AboutModal: React.FC<AboutModalProps> = ({ isOpen, onClose, version }) => {
  if (!isOpen) return null;

  const displayVer = version ? version.replace(/^Native\s+Sirius\s+Core\s+/i, '') : '1.9.8';

  return (
    <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-xs flex items-center justify-center p-4 select-none animate-fadeIn">
      <div className="bg-[#181B20] border border-[#262B34] w-full max-w-md rounded-xl shadow-2xl overflow-hidden text-xs text-slate-200">
        <div className="bg-[#0F1115] text-white px-4 py-3 flex items-center justify-between border-b border-[#262B34]">
          <div className="flex items-center space-x-2">
            <SiriusLogo size={18} showBadge={false} />
            <span className="font-semibold uppercase tracking-wider text-xs">About Sirius Validator Core</span>
          </div>
          <button onClick={onClose} className="p-1 text-slate-400 hover:text-white rounded hover:bg-[#262B34] transition-colors">✕</button>
        </div>

        <div className="p-5 space-y-4">
          <div className="flex items-center space-x-3">
            <div className="p-2.5 rounded-lg bg-[#0F1115] border border-[#262B34] flex-shrink-0">
              <SiriusLogo size={32} />
            </div>
            <div>
              <h3 className="font-semibold text-sm text-white">ProximaX Sirius Mainnet Peer Node</h3>
              <p className="text-slate-400 text-[11px] font-mono">Version {displayVer} (Native Catapult Core)</p>
            </div>
          </div>

          <p className="text-slate-300 text-xs leading-relaxed">
            A high-performance standalone POS validator and distributed storage replicator console for the ProximaX Sirius Mainnet blockchain network.
          </p>

          <div className="p-3 bg-[#0F1115] rounded-lg border border-[#262B34] space-y-1.5 text-[11px] font-mono">
            <div className="flex justify-between">
              <span className="text-slate-400">Network:</span>
              <span className="font-semibold text-blue-400">ProximaX Sirius Mainnet</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Network ID:</span>
              <span className="font-mono text-white">0x60 (96)</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Architecture:</span>
              <span className="font-mono text-white">Native Process Supervisor</span>
            </div>
          </div>

          <div className="space-y-1.5 pt-1 font-medium">
            <a
              href="https://bcdocs.xpxsirius.io/docs/protocol/validating/"
              target="_blank"
              rel="noreferrer"
              className="text-blue-400 hover:underline flex items-center justify-between p-2.5 rounded-lg bg-[#0F1115] border border-[#262B34] hover:bg-[#1E2228] transition-colors"
            >
              <span>Validator Documentation</span>
              <ExternalLink className="w-3.5 h-3.5 text-slate-500" />
            </a>
            <a
              href="https://explorer.xpxsirius.io"
              target="_blank"
              rel="noreferrer"
              className="text-blue-400 hover:underline flex items-center justify-between p-2.5 rounded-lg bg-[#0F1115] border border-[#262B34] hover:bg-[#1E2228] transition-colors"
            >
              <span>Sirius Chain Block Explorer</span>
              <ExternalLink className="w-3.5 h-3.5 text-slate-500" />
            </a>
            <a
              href="https://t.me/proximaxhelpdesk"
              target="_blank"
              rel="noreferrer"
              className="text-blue-400 hover:underline flex items-center justify-between p-2.5 rounded-lg bg-[#0F1115] border border-[#262B34] hover:bg-[#1E2228] transition-colors"
            >
              <span>ProximaX Helpdesk (Telegram)</span>
              <ExternalLink className="w-3.5 h-3.5 text-slate-500" />
            </a>
          </div>
        </div>

        <div className="bg-[#0F1115] px-4 py-2.5 border-t border-[#262B34] flex justify-end">
          <button
            onClick={onClose}
            className="px-4 py-1.5 bg-[#181B20] hover:bg-[#262B34] text-white rounded font-medium text-xs border border-[#262B34] transition-colors"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
