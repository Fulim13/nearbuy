import type { Voucher } from '../entities/Voucher';

export type VoucherState =
  | { kind: 'in-store' }
  | { kind: 'upcoming'; msLeft: number }
  | { kind: 'live'; msLeft: number }
  | { kind: 'sold-out' }
  | { kind: 'ended' };

// Mirrors the admission Lua: window first, then stock. `stock` here is the
// Postgres copy, which lags Redis during a live sale -- so "live" is a hint and
// the seckill call itself is the authority.
export const getVoucherState = (voucher: Voucher, now: number): VoucherState => {
  if (voucher.type !== 'seckill' || !voucher.begin_time || !voucher.end_time)
    return { kind: 'in-store' };

  const begin = Date.parse(voucher.begin_time);
  const end = Date.parse(voucher.end_time);

  if (now < begin) return { kind: 'upcoming', msLeft: begin - now };
  if (now >= end) return { kind: 'ended' };
  if ((voucher.stock ?? 0) <= 0) return { kind: 'sold-out' };
  return { kind: 'live', msLeft: end - now };
};

export const formatDuration = (ms: number) => {
  const total = Math.max(0, Math.floor(ms / 1000));
  const d = Math.floor(total / 86400);
  const h = Math.floor((total % 86400) / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const pad = (n: number) => n.toString().padStart(2, '0');
  return d > 0 ? `${d}d ${pad(h)}:${pad(m)}:${pad(s)}` : `${pad(h)}:${pad(m)}:${pad(s)}`;
};
