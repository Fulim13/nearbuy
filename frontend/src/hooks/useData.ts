import { CanceledError, type AxiosRequestConfig } from 'axios';
import { useEffect, useState } from 'react';
import apiClient from '../services/api-client';
import { getErrorMessage, getStatus } from '../services/errors';

const useData = <T>(
  endpoint: string | null,
  requestConfig?: AxiosRequestConfig,
  deps: unknown[] = []
) => {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState('');
  const [status, setStatus] = useState<number | undefined>();
  const [isLoading, setLoading] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    // A null endpoint means "not ready yet" (e.g. waiting for a route param).
    if (!endpoint) return;

    const controller = new AbortController();
    setLoading(true);
    setError('');

    apiClient
      .get<T>(endpoint, { signal: controller.signal, ...requestConfig })
      .then((res) => {
        setData(res.data);
        setStatus(res.status);
        setLoading(false);
      })
      .catch((err) => {
        if (err instanceof CanceledError) return;
        setError(getErrorMessage(err));
        setStatus(getStatus(err));
        setLoading(false);
      });

    return () => controller.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [endpoint, reloadKey, ...deps]);

  const refetch = () => setReloadKey((key) => key + 1);

  return { data, setData, error, status, isLoading, refetch };
};

export default useData;
