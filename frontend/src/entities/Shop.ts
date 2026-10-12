export interface Shop {
  id: number;
  name: string;
  type_id: number;
  images: string;
  area: string;
  address: string;
  x: number;
  y: number;
  avg_price: number;
  sold: number;
  comments: number;
  score: number;
  open_hours: string;
}

export interface ShopList {
  shops: Shop[];
  total: number;
}
