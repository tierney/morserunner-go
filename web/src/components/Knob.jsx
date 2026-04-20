import React, { useState, useEffect, useRef } from 'react';

const Knob = ({ label, value, min, max, onChange, unit = "" }) => {
  const [isDragging, setIsDragging] = useState(false);
  const knobRef = useRef(null);

  const angle = ((value - min) / (max - min)) * 270 - 135;

  const handleMouseDown = (e) => {
    setIsDragging(true);
    e.preventDefault();
  };

  useEffect(() => {
    const handleMouseMove = (e) => {
      if (!isDragging) return;

      const rect = knobRef.current.getBoundingClientRect();
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;
      
      const angleRad = Math.atan2(e.clientY - centerY, e.clientX - centerX);
      let angleDeg = (angleRad * 180) / Math.PI + 90;
      
      if (angleDeg < 0) angleDeg += 360;
      
      // Map 0-360 to min-max with a dead zone at the bottom
      // 0 deg is top. 45 to 315 is active range (270 degrees)
      let normalizedAngle = angleDeg;
      if (normalizedAngle > 315) normalizedAngle = 315;
      if (normalizedAngle < 45) normalizedAngle = 45;
      
      const val = min + ((normalizedAngle - 45) / 270) * (max - min);
      onChange(Math.round(val));
    };

    const handleMouseUp = () => {
      setIsDragging(false);
    };

    if (isDragging) {
      window.addEventListener('mousemove', handleMouseMove);
      window.addEventListener('mouseup', handleMouseUp);
    }

    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
    };
  }, [isDragging, min, max, onChange]);

  return (
    <div className="flex flex-col items-center gap-2 p-4">
      <div className="text-[10px] uppercase tracking-widest text-secondary font-bold">{label}</div>
      <div 
        ref={knobRef}
        onMouseDown={handleMouseDown}
        className="relative w-16 h-16 cursor-pointer group"
      >
        {/* Knob Background */}
        <svg viewBox="0 0 100 100" className="w-full h-full drop-shadow-lg">
          <circle 
            cx="50" cy="50" r="45" 
            fill="#1a1a20" 
            stroke="rgba(255,255,255,0.1)" 
            strokeWidth="2"
          />
          {/* Progress Arc */}
          <path
            d={`M 20 80 A 42 42 0 1 1 80 80`}
            fill="none"
            stroke="rgba(255,255,255,0.05)"
            strokeWidth="8"
            strokeLinecap="round"
          />
          {/* Pointer */}
          <g transform={`rotate(${angle} 50 50)`}>
            <rect x="48" y="10" width="4" height="15" rx="2" fill="var(--accent-color)" className="filter drop-shadow-[0_0_5px_var(--accent-glow)]" />
          </g>
          {/* Decorative lines */}
          {[...Array(11)].map((_, i) => {
            const a = i * 27 - 135;
            return (
              <line
                key={i}
                x1="50" y1="5" x2="50" y2="8"
                stroke={angle >= a ? "var(--accent-color)" : "rgba(255,255,255,0.2)"}
                strokeWidth="2"
                transform={`rotate(${a} 50 50)`}
              />
            );
          })}
        </svg>
        
        {/* Inner shadow/reflection */}
        <div className="absolute inset-0 rounded-full pointer-events-none bg-gradient-to-br from-white/5 to-black/40" />
      </div>
      <div className="text-sm font-mono neon-text font-bold">
        {value}{unit}
      </div>
      
      <style>{`
        .flex { display: flex; }
        .flex-col { flex-direction: column; }
        .items-center { align-items: center; }
        .gap-2 { gap: 0.5rem; }
        .p-4 { padding: 1rem; }
        .text-[10px] { font-size: 10px; }
        .uppercase { text-transform: uppercase; }
        .tracking-widest { letter-spacing: 0.1em; }
        .text-secondary { color: #9494a0; }
        .font-bold { font-weight: bold; }
        .relative { position: relative; }
        .w-16 { width: 4rem; }
        .h-16 { height: 4rem; }
        .cursor-pointer { cursor: pointer; }
        .w-full { width: 100%; }
        .h-full { height: 100%; }
        .absolute { position: absolute; }
        .inset-0 { top: 0; left: 0; right: 0; bottom: 0; }
        .rounded-full { border-radius: 9999px; }
        .pointer-events-none { pointer-events: none; }
        .text-sm { font-size: 0.875rem; }
        .font-mono { font-family: 'Source Code Pro', monospace; }
      `}</style>
    </div>
  );
};

export default Knob;
