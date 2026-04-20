import React, { useRef, useEffect } from 'react';

const Waterfall = ({ stations, pitch, bw, rit }) => {
  const canvasRef = useRef(null);
  const scrollRef = useRef(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const ctx = canvas.getContext('2d');
    const width = canvas.width;
    const height = canvas.height;

    // Shift waterfall down
    const imageData = ctx.getImageData(0, 0, width, height - 1);
    ctx.putImageData(imageData, 0, 1);
    
    // Clear top row
    ctx.fillStyle = '#0a0a0c';
    ctx.fillRect(0, 0, width, 1);

    // Draw active stations on top row
    // MR frequency range is roughly +/- 500Hz from center (Pitch)
    stations.forEach(st => {
      // bfo is in radians/sample. Convert to Hz.
      // freqHz = (bfo * rate) / (2 * PI)
      const freqHz = (st.bfo * 16000) / (2 * Math.PI);
      
      // Center of waterfall is 'pitch'
      // x-axis: center is width/2. Scale: 1px = 2Hz
      const x = width / 2 + (freqHz - rit) / 2;
      
      if (x >= 0 && x < width) {
        if (st.state === 3) { // StSending
          ctx.fillStyle = '#39FF14';
          ctx.fillRect(x - 2, 0, 4, 1);
        } else {
          ctx.fillStyle = 'rgba(57, 255, 20, 0.2)';
          ctx.fillRect(x - 1, 0, 2, 1);
        }
      }
    });

    // Request next frame
    scrollRef.current = requestAnimationFrame(() => {});
  }, [stations, rit]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const ctx = canvas.getContext('2d');
    const width = canvas.width;
    const height = canvas.height;

    // Overlay static elements (Pitch, Bandwidth)
    // We'll use a separate absolute div for the overlay to avoid clearing the waterfall
  }, []);

  return (
    <div className="relative w-full h-full flex flex-col glass overflow-hidden">
      <div className="p-2 border-b border-white/10 flex justify-between items-center">
        <span className="text-[10px] uppercase tracking-widest font-bold text-secondary">Spectrum Analyzer / Waterfall</span>
        <div className="flex gap-4">
          <span className="text-[10px] text-accent">BW: {bw}Hz</span>
          <span className="text-[10px] text-gold">RIT: {rit}Hz</span>
        </div>
      </div>
      
      <div className="flex-1 relative bg-black/50">
        <canvas 
          ref={canvasRef} 
          width={800} 
          height={400} 
          className="w-full h-full"
        />
        
        {/* Overlay: Pitch & Bandwidth markers */}
        <div className="absolute inset-0 pointer-events-none">
          {/* Center Line (Pitch) */}
          <div className="absolute left-1/2 top-0 bottom-0 w-[1px] bg-white/20" />
          
          {/* Filter Bandwidth Shading */}
          <div 
            className="absolute top-0 bottom-0 bg-accent/5 border-x border-accent/20"
            style={{
              left: `calc(50% - ${bw / 4}px)`,
              width: `${bw / 2}px`
            }}
          />
          
          {/* Station Call Labels */}
          {stations.map(st => {
            const freqHz = (st.bfo * 16000) / (2 * Math.PI);
            const x = 50 + (freqHz - rit) / 4; // Normalized to percent
            return (
              <div 
                key={st.call}
                className="absolute text-[9px] font-mono text-secondary -translate-x-1/2"
                style={{ left: `${x}%`, top: '10px' }}
              >
                {st.call}
              </div>
            );
          })}
        </div>
      </div>
      
      <style>{`
        .bg-accent\\/5 { background-color: rgba(57, 255, 20, 0.05); }
        .bg-accent\\/20 { background-color: rgba(57, 255, 20, 0.2); }
        .border-accent\\/20 { border-color: rgba(57, 255, 20, 0.2); }
      `}</style>
    </div>
  );
};

export default Waterfall;
