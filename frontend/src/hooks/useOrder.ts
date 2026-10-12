import { CanceledError } from 'axios';
import { useEffect, useState } from 'react';
import type { Order } from '../entities/Order';
import apiClient from '../services/api-client';
import { getErrorMessage, getStatus } from '../services/errors';

// After a seckill the gateway answers 202 with an order id BEFORE the Saga has
// written the row, so GET /voucher-order/:id is a 404 for a moment. That 404
// means "pending": poll with backoff. The backoff matters -- the gateway allows
// only 10 requests per user per route per minute.
const BACKOFF_MS = [500, 1000, 1500, 2500, 4000, 6000, 8000];

const useOrder = (id?: string) => {
  const [order, setOrder] = useState<Order | null>(null);
  const [error, setError] = useState('');
  const [isPending, setPending] = useState(false);
  const [isLoading, setLoading] = useState(true);

  useEffect(() => {
    if (!id) return;

    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    let attempt = 0;

    const poll = () => {
      apiClient
        .get<Order>(`/voucher-order/${id}`, { signal: controller.signal })
        .then((res) => {
          setOrder(res.data);
          setPending(false);
          setLoading(false);
        })
        .catch((err) => {
          if (err instanceof CanceledError) return;
          const status = getStatus(err);

          if ((status === 404 || status === 429) && attempt < BACKOFF_MS.length) {
            setPending(true);
            const retryAfter = Number(err.response?.headers['retry-after']);
            const delay =
              status === 429 && retryAfter ? retryAfter * 1000 : BACKOFF_MS[attempt];
            attempt++;
            timer = setTimeout(poll, delay);
            return;
          }

          setPending(false);
          setLoading(false);
          setError(
            status === 404
              ? 'Order not found. If you just grabbed it, the order may have been rolled back (e.g. stock ran out in the database).'
              : getErrorMessage(err)
          );
        });
    };

    setLoading(true);
    setError('');
    poll();

    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [id]);

  return { order, setOrder, error, isPending, isLoading };
};

export default useOrder;
