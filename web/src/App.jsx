import React, { useState, useEffect, useRef } from 'react';
import Knob from './components/Knob';
import Waterfall from './components/Waterfall';
import Log from './components/Log';
import { Power, Activity, Shield, Trophy, Terminal } from 'lucide-react';

function App() {
  const [state, setState] = useState({
    wpm: 30,
    pitch: 600,
    noise: 0.05,
    bw: 500,
    score: 0,
    qsos: 0,
    log: [],
    stations: []
  });
  const [isConnected, setIsConnected] = useState(false);
  const [audioStarted, setAudioStarted] = useState(false);
  const [txText, setTxText] = useState('');
  
  const wsRef = useRef(null);
  const audioCtxRef = useRef(null);
  const audioNodeRef = useRef(null);

  useEffect(() => {
    const ws = new WebSocket(`ws://${window.location.host}/ws`);
    wsRef.current = ws;

    ws.onopen = () => setIsConnected(true);
    ws.onclose = () => setIsConnected(false);
    
    ws.onmessage = async (event) => {
      if (typeof event.data === 'string') {
        const data = JSON.parse(event.data);
        if (data.type === 'state') {
          setState(data);
        }
      } else {
        // Binary audio data
        if (audioNodeRef.current) {
          const buffer = await event.data.arrayBuffer();
          const int16Array = new Int16Array(buffer);
          audioNodeRef.current.port.postMessage(int16Array);
        }
      }
    };

    return () => ws.close();
  }, []);

  const startAudio = async () => {
    if (audioStarted) return;
    
    const ctx = new (window.AudioContext || window.webkitAudioContext)({
      sampleRate: 16000
    });
    await ctx.audioWorklet.addModule('/audio-processor.js');
    
    const node = new AudioWorkletNode(ctx, 'pcm-processor');
    node.connect(ctx.destination);
    
    audioCtxRef.current = ctx;
    audioNodeRef.current = node;
    setAudioStarted(true);
    
    if (ctx.state === 'suspended') {
      await ctx.resume();
    }
  };

  const sendCmd = (cmd, value) => {
    if (wsRef.current && isConnected) {
      wsRef.current.send(JSON.stringify({ cmd, params: { value } }));
    }
  };

  const handleTxSubmit = (e) => {
    e.preventDefault();
    if (txText.trim()) {
      sendCmd('tx', txText);
      setTxText('');
    }
  };

  return (
    <div className="flex flex-col h-full p-4 gap-4">
      {/* Header */}
      <header className="flex justify-between items-center p-4 glass">
        <div className="flex items-center gap-3">
          <div className={`p-2 rounded-lg ${isConnected ? 'bg-accent/20 neon-text' : 'bg-error/20 text-error'}`}>
            <Activity size={24} />
          </div>
          <div>
            <h1 className="text-xl font-bold tracking-tighter">MorseRunner-Go</h1>
            <div className="text-[10px] text-secondary">REAL-TIME ENGINE DASHBOARD v1.0</div>
          </div>
        </div>

        <div className="flex gap-8 items-center">
          <div className="flex flex-col items-end">
            <span className="text-[10px] text-secondary uppercase font-bold">Total Score</span>
            <span className="text-2xl gold-text font-black">{state.score.toLocaleString()}</span>
          </div>
          <div className="flex flex-col items-end">
            <span className="text-[10px] text-secondary uppercase font-bold">QSOs</span>
            <span className="text-2xl neon-text font-black">{state.qsos}</span>
          </div>
          <button 
            onClick={startAudio}
            disabled={audioStarted}
            className={`flex items-center gap-2 px-6 py-2 rounded-full font-bold uppercase text-xs tracking-widest transition-all ${
              audioStarted 
                ? 'bg-accent/10 text-accent border border-accent/30 pointer-events-none' 
                : 'bg-accent text-black hover:scale-105 active:scale-95 shadow-[0_0_20px_var(--accent-glow)]'
            }`}
          >
            <Power size={14} />
            {audioStarted ? 'Audio Online' : 'Initialize Audio'}
          </button>
        </div>
      </header>

      {/* Main Layout */}
      <main className="flex-1 flex gap-4 min-h-0">
        {/* Left Control Panel */}
        <div className="w-64 flex flex-col gap-4">
          <div className="glass p-4 flex flex-col gap-4">
            <h3 className="text-xs font-bold text-secondary flex items-center gap-2">
              <Shield size={14} /> RADIO CONTROLS
            </h3>
            <div className="grid grid-cols-2 gap-2">
              <Knob label="Speed" value={state.wpm} min={15} max={50} unit="WPM" onChange={(v) => sendCmd('wpm', v)} />
              <Knob label="Pitch" value={state.pitch} min={400} max={800} unit="Hz" onChange={(v) => sendCmd('pitch', v)} />
              <Knob label="Noise" value={Math.round(state.noise * 100)} min={0} max={100} unit="%" onChange={(v) => sendCmd('noise', v / 100)} />
              <Knob label="Filter" value={state.bw} min={50} max={1000} unit="Hz" onChange={(v) => sendCmd('bw', v)} />
            </div>
            
            <button 
              onClick={() => sendCmd('pileup', 5)}
              className="mt-2 w-full py-3 bg-white/5 border border-white/10 rounded-lg text-xs font-bold hover:bg-white/10 transition-colors uppercase tracking-widest"
            >
              Start Pile-up
            </button>
          </div>

          <div className="flex-1">
            <Log log={state.log} />
          </div>
        </div>

        {/* Center/Right Visualization Area */}
        <div className="flex-1 flex flex-col gap-4">
          <div className="flex-1">
            <Waterfall 
              stations={state.stations} 
              pitch={state.pitch} 
              bw={state.bw} 
              rit={0} 
            />
          </div>

          {/* TX Input */}
          <div className="glass p-4 flex items-center gap-4">
            <div className="p-2 bg-white/5 rounded text-secondary">
              <Terminal size={18} />
            </div>
            <form onSubmit={handleTxSubmit} className="flex-1 flex gap-2">
              <input 
                type="text" 
                value={txText}
                onChange={(e) => setTxText(e.target.value)}
                placeholder="Enter CW call or exchange (e.g. CQ, K7ABC, 599 001)..."
                className="flex-1 bg-transparent border-b border-white/20 focus:border-accent outline-none text-accent font-mono py-2 tracking-widest"
              />
              <button 
                type="submit"
                className="px-6 py-2 bg-accent/20 border border-accent/40 text-accent font-bold rounded hover:bg-accent/30 transition-all uppercase text-[10px] tracking-widest"
              >
                Send TX
              </button>
            </form>
          </div>
        </div>
      </main>

      {/* Footer / Status Bar */}
      <footer className="glass p-2 px-4 flex justify-between items-center">
        <div className="flex gap-4 text-[10px] text-secondary font-mono">
          <span className="flex items-center gap-1"><div className={`w-2 h-2 rounded-full ${isConnected ? 'bg-accent' : 'bg-error'} pulsing`} /> ENGINE {isConnected ? 'CONNECTED' : 'DISCONNECTED'}</span>
          <span>LATENCY: ~20ms</span>
          <span>SAMPLE RATE: 16000Hz</span>
        </div>
        <div className="text-[10px] text-secondary font-mono">
          &copy; 2026 ANTIGRAVITY ENGINE CORE
        </div>
      </footer>
    </div>
  );
}

export default App;
