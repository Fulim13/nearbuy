import type { ShopType } from '../entities/ShopType';
import useData from './useData';

const useShopTypes = () => useData<ShopType[]>('/shop-types');

export default useShopTypes;
