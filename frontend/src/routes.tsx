import { createBrowserRouter } from 'react-router-dom';
import PrivateRoutes from './components/PrivateRoutes';
import RoleRoutes from './components/RoleRoutes';
import ShopsRedirect from './components/ShopsRedirect';
import CreateShopPage from './pages/CreateShopPage';
import EditShopPage from './pages/EditShopPage';
import ErrorPage from './pages/ErrorPage';
import Layout from './pages/Layout';
import LoginPage from './pages/LoginPage';
import OrderDetailPage from './pages/OrderDetailPage';
import OrdersPage from './pages/OrdersPage';
import ShopDetailPage from './pages/ShopDetailPage';
import ShopListPage from './pages/ShopListPage';
import UserPage from './pages/UserPage';
import UsersPage from './pages/UsersPage';

const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    errorElement: <ErrorPage />,
    children: [
      { index: true, element: <ShopListPage /> },
      { path: 'login', element: <LoginPage /> },
      { path: 'shops', element: <ShopsRedirect /> },
      { path: 'shops/:id', element: <ShopDetailPage /> },
      { path: 'users', element: <UsersPage /> },
      { path: 'users/:id', element: <UserPage /> },
      {
        // Layout route: everything below needs a session.
        element: <PrivateRoutes />,
        children: [
          {
            element: <RoleRoutes allow="admin" />,
            children: [
              { path: 'shops/new', element: <CreateShopPage /> },
              { path: 'shops/:id/edit', element: <EditShopPage /> },
            ],
          },
          {
            element: <RoleRoutes allow="customer" />,
            children: [
              { path: 'orders', element: <OrdersPage /> },
              { path: 'orders/:id', element: <OrderDetailPage /> },
            ],
          },
        ],
      },
    ],
  },
]);

export default router;
