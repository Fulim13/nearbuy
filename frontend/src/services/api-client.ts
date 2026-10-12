import axios, { AxiosError } from 'axios';
import useAuthStore from '../stores/authStore';

// JSON.parse turns 241142716950380545 into 241142716950380540, and the order
// endpoints then answer 404 for an id that was never issued. Quote any integer
// of 16+ digits before parsing so it survives as a string. Safe for this API:
// no string field carries such a number followed by a JSON delimiter.
const quoteBigInts = (raw: string) =>
  raw.replace(/([:,[]\s*)(-?\d{16,})(?=\s*[,}\]])/g, '$1"$2"');

const apiClient = axios.create({
  baseURL: '/api',
  transformResponse: [
    (data) => {
      if (typeof data !== 'string' || data === '') return data;
      try {
        return JSON.parse(quoteBigInts(data));
      } catch {
        return data;
      }
    },
  ],
});

apiClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token;
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

// The session lives 30 minutes in Redis (sliding). If a request that carried a
// token comes back 401, the session is gone: drop it so PrivateRoutes redirects.
apiClient.interceptors.response.use(undefined, (error: AxiosError) => {
  if (error.response?.status === 401 && error.config?.headers?.Authorization)
    useAuthStore.getState().logout();
  return Promise.reject(error);
});

export default apiClient;
