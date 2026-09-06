import React from 'react';

interface SiriusLogoProps {
  className?: string;
  size?: number | string;
  variant?: 'icon' | 'full';
  showBadge?: boolean;
}

export const SiriusLogo: React.FC<SiriusLogoProps> = ({
  className = '',
  size = 28,
  variant = 'icon',
  showBadge = false
}) => {
  const icon = (
    <svg
      width={size}
      height={size}
      viewBox="0 0 90 100"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={`flex-shrink-0 drop-shadow-[0_0_8px_rgba(68,255,241,0.35)] ${className}`}
    >
      <g transform="translate(-76, 418)">
        {/* Polygon 1: Deep Sirius Blue */}
        <path
          d="M105.211 -408.661 L136.461 -372.84 L82.97 -340.674 L79.77 -397.09 Z"
          fill="#0049e3"
          fillRule="evenodd"
        />
        {/* Polygon 2: Science / Electric Blue */}
        <path
          d="M115.235 -415.725 L156.01 -387.771 L142.389 -377.852 L112.426 -414.452 Z"
          fill="#64abff"
          fillRule="evenodd"
        />
        {/* Polygon 3: Bright Neon Cyan Small Top */}
        <path
          d="M153.209 -383.355 L157.299 -354.41 L142.371 -373.952 Z"
          fill="#44fff1"
          fillRule="evenodd"
        />
        {/* Polygon 4: Bright Neon Cyan Lower */}
        <path
          d="M142.041 -367.384 L161.641 -341.205 L127.465 -319.97 L105.541 -334.01 Z"
          fill="#44fff1"
          fillRule="evenodd"
        />
      </g>
    </svg>
  );

  if (variant === 'icon') {
    return icon;
  }

  return (
    <div className={`inline-flex items-center space-x-2.5 ${className}`}>
      {icon}
      <div className="flex flex-col">
        <div className="flex items-center space-x-1.5 leading-none">
          <span className="font-extrabold tracking-wider text-white text-sm">
            SIRIUS
          </span>
          <span className="text-xs font-semibold text-transparent bg-clip-text bg-gradient-to-r from-[#64abff] to-[#44fff1]">
            CHAIN
          </span>
          {showBadge && (
            <span className="text-[9px] font-mono font-bold bg-[#0049e3]/30 text-[#82faf1] px-1.5 py-0.5 rounded border border-[#0049e3]/60">
              MAINNET
            </span>
          )}
        </div>
        <span className="text-[10px] text-gray-400 font-mono tracking-tight mt-0.5">
          Mainnet Peer & Validator
        </span>
      </div>
    </div>
  );
};

export const SiriusWatermark: React.FC<{ className?: string; size?: number }> = ({
  className = '',
  size = 220
}) => (
  <div className={`pointer-events-none absolute right-0 bottom-0 overflow-hidden select-none z-0 ${className}`}>
    <svg
      width={size}
      height={size}
      viewBox="0 0 90 100"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className="transform translate-x-1/4 translate-y-1/4 opacity-[0.06] dark:opacity-[0.09]"
    >
      <g transform="translate(-76, 418)">
        <path
          d="M105.211 -408.661 L136.461 -372.84 L82.97 -340.674 L79.77 -397.09 Z"
          fill="#0049e3"
          fillRule="evenodd"
        />
        <path
          d="M115.235 -415.725 L156.01 -387.771 L142.389 -377.852 L112.426 -414.452 Z"
          fill="#64abff"
          fillRule="evenodd"
        />
        <path
          d="M153.209 -383.355 L157.299 -354.41 L142.371 -373.952 Z"
          fill="#44fff1"
          fillRule="evenodd"
        />
        <path
          d="M142.041 -367.384 L161.641 -341.205 L127.465 -319.97 L105.541 -334.01 Z"
          fill="#44fff1"
          fillRule="evenodd"
        />
      </g>
    </svg>
  </div>
);

