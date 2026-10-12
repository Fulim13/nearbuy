import type { Order } from '../entities/Order';
import useData from './useData';

const useOrders = (limit = 50) =>
  useData<Order[]>('/voucher-orders', { params: { limit } }, [limit]);

export default useOrders;
