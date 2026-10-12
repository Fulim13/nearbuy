export interface Voucher {
  id: number;
  shop_id: number;
  title: string;
  sub_title: string;
  rules: string;
  pay_value: number; // cents
  actual_value: number; // cents
  type: 'normal' | 'seckill';
  status: 'active' | 'inactive' | 'expired';
  // Only present on seckill vouchers.
  stock?: number;
  begin_time?: string;
  end_time?: string;
}
