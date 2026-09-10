import React from 'react';
import { LayoutDashboard, Shield, Network, Sliders, Terminal, Wrench } from 'lucide-react';

interface NavigationTabsProps {
  activeTab: string;
  setActiveTab: (tab: string) => void;
}

export const NavigationTabs: React.FC<NavigationTabsProps> = ({ activeTab, setActiveTab }) => {
  const primaryTabs = [
    { id: 'overview', label: 'Overview', icon: LayoutDashboard },
    { id: 'validator', label: 'Validator', icon: Shield },
    { id: 'network', label: 'Network', icon: Network },
    { id: 'config', label: 'Settings', icon: Sliders },
    { id: 'maintenance', label: 'Maintenance', icon: Wrench },
  ];

  return (
    <nav className="bg-[#0F1115] border-b border-[#262B34] select-none flex-shrink-0">
      <div className="max-w-5xl mx-auto px-4 flex items-center justify-between">
        
        {/* Left: 4 Primary Operator Tabs */}
        <div className="flex items-center space-x-1 sm:space-x-2 overflow-x-auto scrollbar-none">
          {primaryTabs.map((tab) => {
            const Icon = tab.icon;
            const isActive = activeTab === tab.id;

            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`h-11 flex items-center space-x-2 px-3.5 text-xs font-medium border-b-2 -mb-px transition-all whitespace-nowrap ${
                  isActive
                    ? 'border-blue-500 text-white bg-[#181B20]/80'
                    : 'border-transparent text-slate-400 hover:text-slate-200 hover:bg-[#181B20]/40'
                }`}
              >
                <Icon
                  className={`w-3.5 h-3.5 flex-shrink-0 ${
                    isActive ? 'text-blue-400' : 'text-slate-400'
                  }`}
                />
                <span>{tab.label}</span>
              </button>
            );
          })}
        </div>

        {/* Right: De-emphasized Power-User Console */}
        <div className="flex items-center">
          <button
            onClick={() => setActiveTab('logs')}
            className={`h-11 flex items-center space-x-1.5 px-3 text-xs font-mono border-b-2 -mb-px transition-all ${
              activeTab === 'logs'
                ? 'border-slate-400 text-white bg-[#181B20]/80'
                : 'border-transparent text-slate-500 hover:text-slate-300 hover:bg-[#181B20]/40'
            }`}
            title="Developer Console & Live Logs"
          >
            <Terminal className="w-3.5 h-3.5 text-slate-500" />
            <span className="hidden sm:inline">Console</span>
          </button>
        </div>

      </div>
    </nav>
  );
};
