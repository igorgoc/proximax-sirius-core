import React, { useState, useEffect, useRef } from 'react';
import { Folder, FolderOpen, ChevronDown, Check, Archive, Loader2 } from 'lucide-react';

export interface DirectoryItem {
  name: string;
  path: string;
  isDir?: boolean;
  sizeHuman?: string;
}

export interface DirectoryDropdownProps {
  value: string;
  onChange: (newPath: string) => void;
  label?: string;
  placeholder?: string;
  mode?: 'dir' | 'file';
  prompt?: string;
  presets?: DirectoryItem[];
  defaultPresetsOpen?: boolean;
}

const isWin = typeof navigator !== 'undefined' && (/win/i.test(navigator.userAgent || '') || /win/i.test(navigator.platform || ''));

const DEFAULT_DIR_PRESETS: DirectoryItem[] = isWin
  ? [
      { name: 'Default Project Data (./chainconfig/data)', path: './chainconfig/data', isDir: true },
      { name: 'Drive C: Root (C:\\)', path: 'C:\\', isDir: true },
      { name: 'Drive C: (C:\\Sirius_data)', path: 'C:\\Sirius_data', isDir: true },
    ]
  : [
      { name: 'Default Project Data (./chainconfig/data)', path: './chainconfig/data', isDir: true },
      { name: 'External Drive / Volume: /Volumes/SSD/Sirius_data', path: '/Volumes/SSD/Sirius_data', isDir: true },
      { name: 'Browse /Volumes Directory', path: '/Volumes', isDir: true },
    ];

export const DirectoryDropdown: React.FC<DirectoryDropdownProps> = ({
  value,
  onChange,
  label,
  placeholder = isWin ? 'C:\\Sirius_data or ./chainconfig/data' : './chainconfig/data',
  mode = 'dir',
  prompt,
  presets: customPresets,
  defaultPresetsOpen = false,
}) => {
  const isFileMode = mode === 'file';
  const [presetsOpen, setPresetsOpen] = useState(defaultPresetsOpen);
  const [presets, setPresets] = useState<DirectoryItem[]>(() => isFileMode ? [] : DEFAULT_DIR_PRESETS);
  const [browsingNative, setBrowsingNative] = useState(false);
  const isBrowsingRef = useRef(false);

  useEffect(() => {
    let isMounted = true;
    const loadPresets = async () => {
      try {
        const includeParam = isFileMode ? '?includeFiles=true' : '';
        const res = await fetch(`/api/system/browse-dirs${includeParam}`);
        if (res.ok) {
          const data = await res.json();
          if (isMounted && data.presets && Array.isArray(data.presets)) {
            setPresets(data.presets);
          }
        }
      } catch (e) {
        console.error('Failed to load presets:', e);
      }
    };
    loadPresets();
    return () => {
      isMounted = false;
    };
  }, [isFileMode]);

  const availablePresets = customPresets && customPresets.length > 0 ? customPresets : presets;

  const handleNativeBrowse = async () => {
    if (isBrowsingRef.current) return;
    isBrowsingRef.current = true;
    setBrowsingNative(true);
    try {
      const modeParam = mode ? `mode=${encodeURIComponent(mode)}` : 'mode=dir';
      const defaultPrompt = isFileMode ? 'Select Sirius File' : 'Select Sirius Directory';
      const promptParam = `&prompt=${encodeURIComponent(prompt || defaultPrompt)}`;
      const res = await fetch(`/api/system/native-pick-dir?${modeParam}${promptParam}`);
      if (res.ok) {
        const data = await res.json();
        if (data.success && data.path) {
          onChange(data.path);
        }
      }
    } catch (e) {
      console.error('Failed to invoke native picker:', e);
    } finally {
      isBrowsingRef.current = false;
      setBrowsingNative(false);
    }
  };

  return (
    <div className="relative space-y-1.5 w-full">
      {label && (
        <div className="flex items-center justify-between">
          <label className="block font-semibold text-slate-300 text-xs">
            {label}
          </label>
        </div>
      )}

      {/* Input & Action Bar */}
      <div className="flex items-center space-x-1.5 w-full">
        <div className="relative flex-1 min-w-0">
          <div className="absolute inset-y-0 left-0 pl-2.5 flex items-center pointer-events-none text-slate-400">
            {isFileMode ? <Archive className="w-3.5 h-3.5 text-emerald-400" /> : <Folder className="w-3.5 h-3.5 text-slate-400" />}
          </div>
          <input
            type="text"
            value={value}
            onChange={(e) => onChange(e.target.value)}
            placeholder={placeholder}
            title={value || placeholder}
            className="w-full pl-8 pr-2.5 py-1.5 bg-[#0F1115] border border-[#262B34] rounded-md focus:outline-hidden focus:border-blue-500 font-mono text-xs text-slate-100 placeholder-slate-500 truncate"
          />
        </div>

        {/* Native OS Browse Button */}
        <button
          type="button"
          onClick={handleNativeBrowse}
          disabled={browsingNative}
          className="px-2.5 py-1.5 bg-[#0F1115] hover:bg-[#262B34] border border-[#262B34] rounded-md text-xs font-medium text-slate-300 hover:text-white flex items-center space-x-1 transition-colors flex-shrink-0 disabled:opacity-60"
          title={isFileMode ? 'Choose file via system dialog' : 'Choose folder via system dialog'}
        >
          {browsingNative ? (
            <Loader2 className="w-3.5 h-3.5 text-blue-400 animate-spin" />
          ) : isFileMode ? (
            <Archive className="w-3.5 h-3.5 text-emerald-400" />
          ) : (
            <FolderOpen className="w-3.5 h-3.5 text-blue-400" />
          )}
          <span>{browsingNative ? 'Selecting...' : 'Browse'}</span>
        </button>

        {/* Separate Anchored Quick-Access Presets Menu */}
        {availablePresets.length > 0 && (
          <div className="relative flex-shrink-0">
            <button
              type="button"
              onClick={() => setPresetsOpen(!presetsOpen)}
              className="px-2 py-1.5 bg-[#0F1115] hover:bg-[#262B34] border border-[#262B34] rounded-md text-xs font-medium text-slate-400 hover:text-white flex items-center space-x-1 transition-colors"
              title={isFileMode ? 'Snapshot Presets' : 'Quick Presets'}
            >
              <span>Presets</span>
              <ChevronDown className="w-3 h-3" />
            </button>

            {presetsOpen && (
              <>
                {/* Backdrop to dismiss on click outside */}
                <div
                  className="fixed inset-0 z-40"
                  onClick={() => setPresetsOpen(false)}
                />
                <div className="absolute right-0 top-full mt-1 z-50 w-72 bg-[#181B20] border border-[#262B34] rounded-lg shadow-2xl p-1 text-xs">
                  <span className="px-2.5 py-1 text-[10px] uppercase font-semibold text-slate-500 tracking-wider block font-mono">
                    {isFileMode ? 'Detected Snapshots' : 'Suggested Locations & Drives'}
                  </span>
                  <div className="space-y-0.5 mt-0.5 max-h-60 overflow-y-auto">
                    {availablePresets.map((preset, idx) => (
                      <button
                        key={idx}
                        type="button"
                        onClick={() => {
                          onChange(preset.path);
                          setPresetsOpen(false);
                        }}
                        className={`w-full text-left px-2.5 py-1.5 rounded flex items-center justify-between hover:bg-[#262B34] transition-colors ${
                          value === preset.path ? 'bg-blue-600/15 text-blue-400 font-semibold border border-blue-500/30' : 'text-slate-200'
                        }`}
                      >
                        <div className="flex items-center space-x-2 truncate">
                          {preset.isDir === false ? (
                            <Archive className="w-3.5 h-3.5 text-emerald-400 flex-shrink-0" />
                          ) : (
                            <Folder className="w-3.5 h-3.5 text-slate-400 flex-shrink-0" />
                          )}
                          <span className="truncate text-[11px] font-mono">{preset.name}</span>
                        </div>
                        {value === preset.path && <Check className="w-3.5 h-3.5 text-blue-400 flex-shrink-0" />}
                      </button>
                    ))}
                  </div>
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
};
