import React from 'react';

const Log = ({ log }) => {
  return (
    <div className="glass flex flex-col h-full overflow-hidden">
      <div className="p-2 border-b border-white/10 bg-white/5">
        <span className="text-[10px] uppercase tracking-widest font-bold text-secondary">Contest Log</span>
      </div>
      <div className="flex-1 overflow-y-auto p-2">
        <table className="w-full text-left text-xs font-mono">
          <thead className="text-secondary border-b border-white/5">
            <tr>
              <th className="pb-1">TIME</th>
              <th className="pb-1">CALL</th>
              <th className="pb-1">PTS</th>
              <th className="pb-1">MULT</th>
            </tr>
          </thead>
          <tbody>
            {[...log].reverse().map((q, i) => (
              <tr key={i} className="border-b border-white/5 last:border-0 hover:bg-white/5">
                <td className="py-1 text-secondary">{new Date(q.Timestamp).toLocaleTimeString([], { hour12: false })}</td>
                <td className="py-1 neon-text font-bold">{q.Call}</td>
                <td className="py-1 text-gold">{q.Points}</td>
                <td className="py-1 text-white">{q.Mult ? 'YES' : 'NO'}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {log.length === 0 && (
          <div className="h-full flex items-center justify-center text-secondary italic opacity-50">
            No QSOs yet. Start a pile-up!
          </div>
        )}
      </div>
    </div>
  );
};

export default Log;
