import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User } from '../entities/User';

interface AuthStore {
  token: string | null;
  user: User | null;
  login: (token: string, user: User) => void;
  logout: () => void;
}

// Persisted so a page refresh keeps the session; the token itself is still
// validated by the gateway on every request.
const useAuthStore = create<AuthStore>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      login: (token, user) => set(() => ({ token, user })),
      logout: () => set(() => ({ token: null, user: null })),
    }),
    { name: 'nearbuy-auth' }
  )
);

// Admins run the catalogue (shops, vouchers); they cannot grab, pay or follow.
// The gateway enforces this; the UI only hides what would be refused.
export const useIsAdmin = () => useAuthStore((s) => s.user?.role === 'admin');

export default useAuthStore;
