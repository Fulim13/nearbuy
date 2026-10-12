import type { Voucher } from '../entities/Voucher';
import useData from './useData';

const useVouchers = (shopId?: string | number) =>
  useData<Voucher[]>(shopId ? `/shops/${shopId}/vouchers` : null);

export default useVouchers;
