import React, { useState, useEffect, useRef, useMemo } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import {
  Terminal,
  Download,
  Trash2,
  Pause,
  Play,
  Search,
  ArrowDown,
  Copy,
  Check,
  MoreVertical,
  CornerDownLeft,
  X
} from 'lucide-react';

interface RawLogRecord {
  id: number;
  raw: string;
}

interface ParsedLogEntry {
  id: string;
  raw: string;
  cleanText: string;
  timestamp: string;
  level: 'info' | 'warn' | 'error' | 'debug';
  category: 'consensus' | 'p2p' | 'blocks' | 'system';
  component: string;
  message: string;
  count: number;
  lastSeen: string;
}

type LogCategory = 'all' | 'consensus' | 'p2p' | 'errors';

// Strip ANSI terminal escape sequences
const stripAnsi = (str: string): string => {
  return str.replace(/\u001b\[[0-9;]*m/g, '').replace(/[\x00-\x09\x0B-\x1F\x7F]/g, '');
};

// Parse raw Sirius C++ log line into structured entity
const parseLogLine = (record: RawLogRecord): ParsedLogEntry => {
  const clean = stripAnsi(record.raw).trim();
  
  let level: 'info' | 'warn' | 'error' | 'debug' = 'info';
  const lower = clean.toLowerCase();
  if (
    clean.includes('<error>') ||
    clean.includes('ERR') ||
    clean.includes('Failure') ||
    clean.includes('FATAL') ||
    lower.includes('error') ||
    lower.includes('failed') ||
    lower.includes('failure') ||
    lower.includes('exit status') ||
    lower.includes('lockopen') ||
    lower.includes('exception') ||
    lower.includes('crash')
  ) {
    level = 'error';
  } else if (clean.includes('<warning>') || clean.includes('WARN') || lower.includes('warning')) {
    level = 'warn';
  } else if (clean.includes('<debug>')) {
    level = 'debug';
  }

  let category: 'consensus' | 'p2p' | 'blocks' | 'system' = 'system';
  if (lower.includes('dbrb') || lower.includes('finality') || lower.includes('commit') || lower.includes('prepare') || lower.includes('harvest')) {
    category = 'consensus';
  } else if (lower.includes('peer') || lower.includes('packet') || lower.includes('ping') || lower.includes('socket') || lower.includes('connector')) {
    category = 'p2p';
  } else if (lower.includes('block') || lower.includes('height') || lower.includes('pull') || lower.includes('consumer') || lower.includes('disruptor')) {
    category = 'blocks';
  }

  const timeMatch = clean.match(/(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)/);
  let timestamp = '';
  if (timeMatch) {
    const rawTimeStr = timeMatch[1].replace(' ', 'T') + 'Z';
    const d = new Date(rawTimeStr);
    if (!isNaN(d.getTime())) {
      const h = String(d.getHours()).padStart(2, '0');
      const m = String(d.getMinutes()).padStart(2, '0');
      const s = String(d.getSeconds()).padStart(2, '0');
      timestamp = `${h}:${m}:${s}`;
    } else {
      timestamp = timeMatch[1].split(' ')[1] || timeMatch[1];
    }
  }

  const compMatch = clean.match(/\(([a-zA-Z0-9_:]+\.[a-zA-Z0-9]+@\d+)\)/);
  const component = compMatch ? compMatch[1] : '';

  let message = clean;
  if (compMatch) {
    const splitIndex = clean.indexOf(compMatch[0]);
    if (splitIndex !== -1) {
      message = clean.substring(splitIndex + compMatch[0].length).trim();
    }
  } else if (timeMatch) {
    message = clean.replace(timeMatch[0], '').trim();
  }

  return {
    id: `log-${record.id}`,
    raw: record.raw,
    cleanText: clean,
    timestamp: timestamp || new Date().toLocaleTimeString(),
    level,
    category,
    component,
    message: message || clean,
    count: 1,
    lastSeen: timestamp || new Date().toLocaleTimeString()
  };
};

export const LogsTab: React.FC = () => {
  const [rawLogs, setRawLogs] = useState<RawLogRecord[]>([]);
  const [activeCategory, setActiveCategory] = useState<LogCategory>('all');
  const [filter, setFilter] = useState('');
  const [isPaused, setIsPaused] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [overflowOpen, setOverflowOpen] = useState(false);

  // Interactive CLI prompt
  const [cliInput, setCliInput] = useState('');
  const [cliOutput, setCliOutput] = useState<string | null>(null);
  const [executingCli, setExecutingCli] = useState(false);

  const logContainerRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const queueRef = useRef<RawLogRecord[]>([]);
  const logIdCounterRef = useRef(0);

  // Initial fetch of logs
  useEffect(() => {
    fetch('/api/logs')
      .then((res) => res.json())
      .then((data) => {
        if (data && data.logs && Array.isArray(data.logs)) {
          const records: RawLogRecord[] = data.logs.slice(-1000).map((line: string) => ({
            id: ++logIdCounterRef.current,
            raw: line,
          }));
          setRawLogs(records);
        }
      })
      .catch(console.error);

    // High-efficiency batched flush (4 flushes per second to prevent React render churn)
    const flushInterval = setInterval(() => {
      if (queueRef.current.length > 0 && !isPaused && !document.hidden) {
        const incoming = queueRef.current;
        queueRef.current = [];
        setRawLogs((prev) => [...prev.slice(-(1000 - incoming.length)), ...incoming]);
      }
    }, 250);

    // WebSocket live stream
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/logs/stream`;
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onmessage = (event) => {
      if (!isPaused && !document.hidden) {
        queueRef.current.push({
          id: ++logIdCounterRef.current,
          raw: event.data,
        });
      }
    };

    ws.onerror = () => {
      console.warn('WebSocket log connection unconfigured or closed');
    };

    return () => {
      clearInterval(flushInterval);
      ws.close();
    };
  }, [isPaused]);

  // Auto-collapse repeated log entries at render time
  const structuredLogs = useMemo(() => {
    const parsed = rawLogs.map((record) => parseLogLine(record));
    const collapsed: ParsedLogEntry[] = [];
    
    for (const entry of parsed) {
      if (collapsed.length > 0) {
        const prev = collapsed[collapsed.length - 1];
        if (prev.component === entry.component && prev.message === entry.message) {
          prev.count += 1;
          prev.lastSeen = entry.timestamp;
          continue;
        }
      }
      collapsed.push({ ...entry });
    }
    return collapsed;
  }, [rawLogs]);

  // Filter logs based on category and search
  const displayedLogs = useMemo(() => {
    return structuredLogs.filter((entry) => {
      if (activeCategory === 'consensus' && entry.category !== 'consensus' && entry.category !== 'blocks') {
        return false;
      }
      if (activeCategory === 'p2p' && entry.category !== 'p2p') {
        return false;
      }
      if (activeCategory === 'errors' && entry.level !== 'error' && entry.level !== 'warn') {
        return false;
      }

      if (filter) {
        const query = filter.toLowerCase();
        return (
          entry.cleanText.toLowerCase().includes(query) ||
          entry.component.toLowerCase().includes(query) ||
          entry.message.toLowerCase().includes(query)
        );
      }
      return true;
    });
  }, [structuredLogs, activeCategory, filter]);

  // Virtualizer: dynamic element measurement for variable-height wrapped lines
  const rowVirtualizer = useVirtualizer({
    count: displayedLogs.length,
    getScrollElement: () => logContainerRef.current,
    estimateSize: () => 28,
    getItemKey: (index) => displayedLogs[index]?.id ?? index,
    overscan: 12,
  });

  const lastLogEntry = displayedLogs[displayedLogs.length - 1];
  const lastLogKey = lastLogEntry ? `${lastLogEntry.id}-${lastLogEntry.count}` : '';

  // Scroll to bottom when autoScroll is enabled
  useEffect(() => {
    if (autoScroll && !isPaused && displayedLogs.length > 0) {
      rowVirtualizer.scrollToIndex(displayedLogs.length - 1, { align: 'end' });
    }
  }, [lastLogKey, autoScroll, isPaused]);

  // Detect user scrolling upwards
  const handleScroll = () => {
    if (!logContainerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = logContainerRef.current;
    const isNearBottom = scrollHeight - scrollTop - clientHeight < 60;
    if (!isNearBottom && autoScroll) {
      setAutoScroll(false);
    } else if (isNearBottom && !autoScroll) {
      setAutoScroll(true);
    }
  };

  const errorCount = useMemo(() => {
    return structuredLogs.filter((l) => l.level === 'error').length;
  }, [structuredLogs]);

  const handleDownload = () => {
    const element = document.createElement('a');
    const file = new Blob([rawLogs.map((r) => stripAnsi(r.raw)).join('\n')], { type: 'text/plain' });
    element.href = URL.createObjectURL(file);
    element.download = `sirius-node-${new Date().toISOString().slice(0, 10)}.log`;
    document.body.appendChild(element);
    element.click();
    document.body.removeChild(element);
    setOverflowOpen(false);
  };

  const handleClearLogs = () => {
    queueRef.current = [];
    setRawLogs([]);
    setOverflowOpen(false);
  };

  const handleCopyLine = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  const handleExecuteCli = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!cliInput.trim()) return;
    setExecutingCli(true);
    try {
      const endpoint = cliInput.trim().startsWith('/') ? cliInput.trim() : `/${cliInput.trim()}`;
      const res = await fetch(`/api/console/query?endpoint=${encodeURIComponent(endpoint)}`);
      const data = await res.json();
      setCliOutput(JSON.stringify(data, null, 2));
    } catch (err: any) {
      setCliOutput(`Error: ${err.message || 'Failed to query endpoint'}`);
    } finally {
      setExecutingCli(false);
    }
  };

  return (
    <div className="space-y-4 max-w-6xl mx-auto px-4 py-6 select-none h-[calc(100vh-125px)] flex flex-col">
      
      {/* 1. Minimal Toolbar: Primary Controls Only */}
      <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-3 flex flex-col md:flex-row md:items-center justify-between gap-3 flex-shrink-0">
        
        {/* Category Selector */}
        <div className="flex items-center space-x-1 bg-[#0F1115] p-1 rounded-md border border-[#262B34]">
          <button
            onClick={() => setActiveCategory('all')}
            className={`px-2.5 py-1 text-xs font-medium rounded transition-colors ${
              activeCategory === 'all' ? 'bg-[#262B34] text-white' : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            All Activity
          </button>
          <button
            onClick={() => setActiveCategory('consensus')}
            className={`px-2.5 py-1 text-xs font-medium rounded transition-colors ${
              activeCategory === 'consensus' ? 'bg-[#262B34] text-white' : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            Consensus
          </button>
          <button
            onClick={() => setActiveCategory('p2p')}
            className={`px-2.5 py-1 text-xs font-medium rounded transition-colors ${
              activeCategory === 'p2p' ? 'bg-[#262B34] text-white' : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            P2P
          </button>
          <button
            onClick={() => setActiveCategory('errors')}
            className={`px-2.5 py-1 text-xs font-medium rounded transition-colors ${
              activeCategory === 'errors' ? 'bg-[#262B34] text-rose-400' : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            Errors ({errorCount})
          </button>
        </div>

        {/* Primary Controls & Search */}
        <div className="flex items-center space-x-2">
          {/* Search Box */}
          <div className="relative">
            <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-2.5" />
            <input
              type="text"
              placeholder="Search logs..."
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="bg-[#0F1115] border border-[#262B34] rounded px-2.5 pl-8 py-1.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500 font-mono w-48 sm:w-60"
            />
            {filter && (
              <button
                onClick={() => setFilter('')}
                className="absolute right-2 top-2 text-slate-500 hover:text-slate-300"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          {/* Pause / Resume Button */}
          <button
            onClick={() => setIsPaused(!isPaused)}
            className={`px-3 py-1.5 rounded text-xs font-medium border flex items-center space-x-1.5 transition-colors ${
              isPaused
                ? 'bg-amber-950/40 border-amber-800/60 text-amber-300'
                : 'bg-[#0F1115] border-[#262B34] text-slate-300 hover:bg-[#262B34]'
            }`}
            title={isPaused ? 'Resume live log stream' : 'Pause live log stream'}
          >
            {isPaused ? <Play className="w-3.5 h-3.5 fill-current" /> : <Pause className="w-3.5 h-3.5" />}
            <span>{isPaused ? 'Resume' : 'Pause'}</span>
          </button>

          {/* Follow Tail Toggle */}
          <button
            onClick={() => {
              setAutoScroll(!autoScroll);
              if (!autoScroll && displayedLogs.length > 0) {
                rowVirtualizer.scrollToIndex(displayedLogs.length - 1, { align: 'end' });
              }
            }}
            className={`px-2.5 py-1.5 rounded text-xs font-medium border transition-colors ${
              autoScroll
                ? 'bg-[#262B34] border-[#38414E] text-white'
                : 'bg-[#0F1115] border-[#262B34] text-slate-400 hover:text-slate-200'
            }`}
            title="Keep scrolling to bottom on new incoming entries"
          >
            Follow
          </button>

          {/* Overflow Menu (⋯) for Infrequent Actions */}
          <div className="relative">
            <button
              onClick={() => setOverflowOpen(!overflowOpen)}
              className="p-1.5 bg-[#0F1115] hover:bg-[#262B34] border border-[#262B34] rounded text-slate-400 hover:text-white transition-colors"
              title="More actions"
            >
              <MoreVertical className="w-3.5 h-3.5" />
            </button>

            {overflowOpen && (
              <>
                <div className="fixed inset-0 z-30" onClick={() => setOverflowOpen(false)} />
                <div className="absolute right-0 top-full mt-1 z-40 w-44 bg-[#181B20] border border-[#262B34] rounded-lg shadow-2xl p-1 text-xs font-mono">
                  <button
                    onClick={handleDownload}
                    className="w-full text-left px-2.5 py-1.5 hover:bg-[#262B34] rounded text-slate-300 flex items-center space-x-2 transition-colors"
                  >
                    <Download className="w-3.5 h-3.5 text-blue-400" />
                    <span>Download Log</span>
                  </button>
                  <button
                    onClick={handleClearLogs}
                    className="w-full text-left px-2.5 py-1.5 hover:bg-[#262B34] rounded text-rose-400 flex items-center space-x-2 transition-colors"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                    <span>Clear Terminal</span>
                  </button>
                </div>
              </>
            )}
          </div>

        </div>
      </section>

      {/* 2. Virtualized Log Viewer Terminal */}
      <div className="relative flex-1 bg-[#0F1115] border border-[#262B34] rounded-lg overflow-hidden flex flex-col">
        
        {/* Floating "Auto-scroll paused" indicator: ONLY shows if there is actual scrollable content */}
        {!autoScroll && displayedLogs.length > 10 && (
          <button
            onClick={() => {
              setAutoScroll(true);
              rowVirtualizer.scrollToIndex(displayedLogs.length - 1, { align: 'end' });
            }}
            className="absolute bottom-4 right-4 z-20 px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-md text-xs font-medium shadow-lg flex items-center space-x-1.5 animate-bounce transition-colors"
          >
            <ArrowDown className="w-3.5 h-3.5" />
            <span>Scroll to Live End</span>
          </button>
        )}

        {/* Virtualized Container */}
        <div
          ref={logContainerRef}
          onScroll={handleScroll}
          className="flex-1 overflow-y-auto p-3 font-mono text-xs scrollbar-thin select-text"
        >
          {displayedLogs.length > 0 ? (
            <div
              style={{
                height: `${rowVirtualizer.getTotalSize()}px`,
                width: '100%',
                position: 'relative',
              }}
            >
              {rowVirtualizer.getVirtualItems().map((virtualRow) => {
                const log = displayedLogs[virtualRow.index];
                if (!log) return null;

                const isError = log.level === 'error';
                const isWarn = log.level === 'warn';

                return (
                  <div
                    key={virtualRow.key}
                    data-index={virtualRow.index}
                    ref={rowVirtualizer.measureElement}
                    style={{
                      position: 'absolute',
                      top: 0,
                      left: 0,
                      width: '100%',
                      transform: `translateY(${virtualRow.start}px)`,
                    }}
                    className={`py-0.5 px-1.5 rounded flex items-baseline space-x-2 hover:bg-[#181B20] group transition-colors leading-relaxed ${
                      isError
                        ? 'bg-rose-950/20 text-rose-300'
                        : isWarn
                        ? 'bg-amber-950/15 text-amber-300'
                        : 'text-slate-300'
                    }`}
                  >
                    {/* Timestamp */}
                    <span className="text-slate-500 text-[11px] flex-shrink-0 select-none cursor-default" title="Local Time">
                      {log.timestamp}
                    </span>

                    {/* Component */}
                    {log.component && (
                      <span className="text-blue-400/80 text-[11px] flex-shrink-0">
                        [{log.component.split('@')[0]}]
                      </span>
                    )}

                    {/* Message Body */}
                    <span className="break-words whitespace-pre-wrap flex-1 min-w-0">
                      {log.message}
                    </span>

                    {/* Render-Time Auto-Collapse Badge (message x14) */}
                    {log.count > 1 && (
                      <span className="bg-[#262B34] text-emerald-400 font-bold px-1.5 py-0.2 rounded text-[10px] flex-shrink-0" title={`Repeated ${log.count} times, last at ${log.lastSeen}`}>
                        ×{log.count}
                      </span>
                    )}

                    {/* Copy Button on Hover */}
                    <button
                      onClick={() => handleCopyLine(log.cleanText, log.id)}
                      className="opacity-0 group-hover:opacity-100 p-0.5 hover:bg-[#262B34] rounded text-slate-500 hover:text-slate-200 transition-opacity flex-shrink-0"
                      title="Copy log line"
                    >
                      {copiedId === log.id ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                    </button>
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="py-20 text-center text-slate-500 text-xs">
              <Terminal className="w-6 h-6 mx-auto mb-2 text-slate-600" />
              <p>No log messages match the current filter.</p>
              <p className="text-[11px] text-slate-600 mt-1">Sirius engine stream is listening...</p>
            </div>
          )}
        </div>

      </div>

      {/* 3. Bitcoin Core-Style Interactive RPC Diagnostic Query Console */}
      <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-3 flex-shrink-0">
        <form onSubmit={handleExecuteCli} className="flex items-center space-x-2">
          <span className="text-slate-500 font-mono text-xs pl-1">sirius-cli &gt;</span>
          <input
            type="text"
            placeholder="/chain/height, /node/info, /chain/score..."
            value={cliInput}
            onChange={(e) => setCliInput(e.target.value)}
            className="flex-1 bg-[#0F1115] border border-[#262B34] rounded px-2.5 py-1.5 text-xs text-slate-200 font-mono focus:outline-none focus:border-blue-500"
          />
          <button
            type="submit"
            disabled={executingCli || !cliInput.trim()}
            className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-40 text-white rounded text-xs font-semibold flex items-center space-x-1 transition-colors"
          >
            <span>Query</span>
            <CornerDownLeft className="w-3 h-3" />
          </button>
        </form>

        {cliOutput && (
          <div className="mt-2.5 p-2 bg-[#0F1115] border border-[#262B34] rounded font-mono text-xs text-emerald-400 max-h-36 overflow-y-auto relative">
            <button
              onClick={() => setCliOutput(null)}
              className="absolute top-1.5 right-1.5 text-slate-500 hover:text-slate-300"
              title="Close Output"
            >
              <X className="w-3.5 h-3.5" />
            </button>
            <pre className="whitespace-pre-wrap">{cliOutput}</pre>
          </div>
        )}
      </section>

    </div>
  );
};
