import { AxiosError } from 'axios';

interface ApiErrorBody {
  message?: string;
}

// Backend messages are lowercase ("already ordered"); they read as sentences here.
const capitalize = (text: string) => text.charAt(0).toUpperCase() + text.slice(1);

export const getErrorMessage = (err: unknown): string => {
  if (err instanceof AxiosError) {
    const res = err.response;
    if (!res) return err.message;

    if (res.status === 429) {
      const retry = res.headers['retry-after'];
      return `Too many requests. Try again in ${retry ?? 'a few'} seconds.`;
    }
    const body = res.data as ApiErrorBody | string;
    if (typeof body === 'object' && body?.message) return capitalize(body.message);
    return `Request failed (${res.status})`;
  }
  if (err instanceof Error) return err.message;
  return 'Something went wrong';
};

export const getStatus = (err: unknown) =>
  err instanceof AxiosError ? err.response?.status : undefined;
