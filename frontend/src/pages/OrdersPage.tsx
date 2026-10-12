import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import ReceiptLongIcon from '@mui/icons-material/ReceiptLong';
import {
  Alert,
  Box,
  Button,
  Card,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Skeleton,
  Stack,
  Tab,
  Tabs,
  Typography,
} from '@mui/material';
import { useState } from 'react';
import { Link } from 'react-router-dom';
import OrderStatusChip from '../components/OrderStatusChip';
import type { OrderStatus } from '../entities/Order';
import useOrders from '../hooks/useOrders';
import { formatCents, formatDateTime } from '../utils/money';

type Filter = 'all' | OrderStatus;

const filters: Filter[] = ['all', 'unpaid', 'paid', 'cancelled'];

const OrdersPage = () => {
  const { data: orders, error, isLoading, refetch } = useOrders();
  const [filter, setFilter] = useState<Filter>('all');

  const visible = orders?.filter((o) => filter === 'all' || o.status === filter);

  return (
    <>
      <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Typography variant="h4">My orders</Typography>
        <Button onClick={refetch}>Refresh</Button>
      </Stack>

      <Tabs value={filter} onChange={(_, value) => setFilter(value)} sx={{ mb: 2 }}>
        {filters.map((f) => (
          <Tab key={f} value={f} label={f.charAt(0).toUpperCase() + f.slice(1)} />
        ))}
      </Tabs>

      {error && <Alert severity="error">{error}</Alert>}
      {isLoading && <Skeleton variant="rounded" height={240} />}
      {!isLoading && visible?.length === 0 && (
        <Alert severity="info">
          No orders here yet. Find a shop with a live flash sale and grab a voucher.
        </Alert>
      )}

      {!isLoading && !!visible?.length && (
        <Card>
          <List disablePadding>
            {visible.map((order) => (
              <ListItemButton key={order.id} component={Link} to={`/orders/${order.id}`} divider>
                <ListItemIcon>
                  <ReceiptLongIcon color="primary" />
                </ListItemIcon>
                <ListItemText
                  primary={`Order #${order.id}`}
                  secondary={`Voucher #${order.voucher_id} · ${formatDateTime(order.create_time)}`}
                />
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Typography sx={{ fontWeight: 700 }}>{formatCents(order.pay_value)}</Typography>
                  <OrderStatusChip status={order.status} />
                  <ChevronRightIcon color="action" />
                </Box>
              </ListItemButton>
            ))}
          </List>
        </Card>
      )}
    </>
  );
};

export default OrdersPage;
