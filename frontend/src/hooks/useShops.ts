import type { ShopList } from '../entities/Shop';
import useData from './useData';

export interface ShopQuery {
  typeId?: number;
  name?: string;
  page: number;
  pageSize: number;
}

// No filter lists every shop; type_id or name narrows it.
const useShops = (query: ShopQuery) =>
  useData<ShopList>(
    '/shops',
    {
      params: {
        type_id: query.typeId || undefined,
        name: query.name || undefined,
        page: query.page,
        page_size: query.pageSize,
      },
    },
    [query.typeId, query.name, query.page, query.pageSize]
  );

export default useShops;
