import { Navigate, Outlet } from 'react-router-dom';
import useAuthStore from '../stores/authStore';

interface Props {
  allow: 'admin' | 'customer';
}

// Sits inside PrivateRoutes, so a user is always present here.
const RoleRoutes = ({ allow }: Props) => {
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === 'admin';

  if (allow === 'admin' ? !isAdmin : isAdmin) return <Navigate to="/" replace />;

  return <Outlet />;
};

export default RoleRoutes;
