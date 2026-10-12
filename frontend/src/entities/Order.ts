export type OrderStatus =
  | 'unpaid'
  | 'paid'
  | 'redeemed'
  | 'cancelled'
  | 'refunding'
  | 'refunded';

// Order ids come from pkg/idgen (seconds << 32 | counter), which is far beyond
// Number.MAX_SAFE_INTEGER. api-client keeps them as strings; never Number() them.
export interface Order {
  id: string;
  user_id: number;
  voucher_id: number;
  pay_value: number; // cents
  pay_type: 'balance' | 'card';
  status: OrderStatus;
  create_time: string;
  pay_time?: string;
  use_time?: string;
  refund_time?: string;
}

export interface SeckillResult {
  order_id: string;
  status: 'pending';
}
