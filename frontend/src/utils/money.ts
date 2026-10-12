// Vouchers and orders are stored in cents; shop avg_price is whole ringgit.
export const formatCents = (cents: number) => `RM ${(cents / 100).toFixed(2)}`;

export const formatRinggit = (amount: number) => `RM ${amount}`;

export const formatDateTime = (iso?: string) =>
  iso ? new Date(iso).toLocaleString() : '—';
