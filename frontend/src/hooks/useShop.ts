import type { Shop } from '../entities/Shop';
import useData from './useData';

const useShop = (id?: string) => useData<Shop>(id ? `/shops/${id}` : null);

export default useShop;
