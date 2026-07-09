import { motion } from 'framer-motion';

interface TabsProps<T extends string> {
  options: { value: T; label: string; count?: number }[];
  value: T;
  onChange: (v: T) => void;
}

export function Tabs<T extends string>({ options, value, onChange }: TabsProps<T>) {
  return (
    <div className="flex gap-1 border-b border-white/[0.08] mb-3">
      {options.map((o) => {
        const isActive = o.value === value;
        return (
          <button
            key={o.value}
            onClick={() => onChange(o.value)}
            className={`relative px-3 py-2 text-xs transition-colors ${
              isActive ? 'text-accent' : 'text-text-2 hover:text-text-0'
            }`}
          >
            {o.label}
            {typeof o.count === 'number' && (
              <span className="text-text-3 ml-1.5 tabular-nums">{o.count}</span>
            )}
            {isActive && (
              <motion.div
                layoutId="peer-tab-indicator"
                transition={{ type: 'spring', stiffness: 380, damping: 32 }}
                className="absolute bottom-0 left-1 right-1 h-[2px] bg-accent"
              />
            )}
          </button>
        );
      })}
    </div>
  );
}
