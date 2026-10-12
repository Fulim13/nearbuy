import type { ShopList } from '../entities/Shop';
import useData from './useData';

export interface ShopQuery {
  typeId?: number;
  name?: string;
  page: number;
  pageSize: number;
}

const useShops = (query: ShopQuery) => {
  const hasFilter = !!query.typeId || !!query.name;

  return useData<ShopList>(
    hasFilter ? '/shops' : null,
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
};

export default useShops;
