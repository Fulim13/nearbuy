import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import HourglassTopIcon from '@mui/icons-material/HourglassTop';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  Divider,
  Stack,
  Typography,
} from '@mui/material';
import { useState, type ReactNode } from 'react';
import { Link, useParams } from 'react-router-dom';
import OrderStatusChip from '../components/OrderStatusChip';
import PageQrCode from '../components/PageQrCode';
import PaymentDialog from '../components/PaymentDialog';
import type { Voucher } from '../entities/Voucher';
import useData from '../hooks/useData';
import useNow from '../hooks/useNow';
import useOrder from '../hooks/useOrder';
import { formatCents, formatDateTime } from '../utils/money';
import { formatDuration } from '../utils/voucherState';

// order-service cancels unpaid orders after ORDER_EXPIRY (15m in deploy/dev).
const ORDER_EXPIRY_MS = 15 * 60_000;

const Row = ({ label, value }: { label: string; value: ReactNode }) => (
  <Stack direction="row" sx={{ justifyContent: 'space-between', py: 0.75 }}>
    <Typography color="text.secondary">{label}</Typography>
    <Typography component="div" sx={{ fontWeight: 600 }}>
      {value}
    </Typography>
  </Stack>
);

const OrderDetailPage = () => {
  const { id } = useParams();
  const now = useNow();
  const { order, setOrder, error, isPending, isLoading } = useOrder(id);
  const { data: voucher } = useData<Voucher>(order ? `/vouchers/${order.voucher_id}` : null);
  const [isPaymentOpen, setPaymentOpen] = useState(false);

  if (isPending || (isLoading && !error))
    return (
      <Card sx={{ maxWidth: 560, mx: 'auto' }}>
        <CardContent sx={{ p: 5, textAlign: 'center' }}>
          {isPending ? <HourglassTopIcon color="primary" sx={{ fontSize: 48 }} /> : <CircularProgress />}
          <Typography variant="h6" sx={{ mt: 2 }}>
            {isPending ? 'You got one! Creating your order…' : 'Loading order…'}
          </Typography>
          {isPending && (
            <>
              <Typography color="text.secondary" sx={{ mt: 1 }}>
                Your spot is reserved. The order is being written by the saga; this usually takes a second.
              </Typography>
              <CircularProgress size={24} sx={{ mt: 3 }} />
            </>
          )}
        </CardContent>
      </Card>
    );

  if (error || !order)
    return (
      <Stack spacing={2} sx={{ maxWidth: 560, mx: 'auto' }}>
        <Alert severity="error">{error}</Alert>
        <Button component={Link} to="/orders">Back to my orders</Button>
      </Stack>
    );

  const expiresIn = Date.parse(order.create_time) + ORDER_EXPIRY_MS - now;

  return (
    <Card sx={{ maxWidth: 560, mx: 'auto' }}>
      <CardContent sx={{ p: 4 }}>
        <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'center' }}>
          <Typography variant="h5">Order details</Typography>
          <OrderStatusChip status={order.status} />
        </Stack>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          #{order.id}
        </Typography>

        {voucher && (
          <Box sx={{ bgcolor: '#FFF4E6', borderRadius: 2, p: 2, mb: 2 }}>
            <Typography sx={{ fontWeight: 700 }}>{voucher.title}</Typography>
            {voucher.sub_title && (
              <Typography variant="body2" color="text.secondary">{voucher.sub_title}</Typography>
            )}
            <Button component={Link} to={`/shops/${voucher.shop_id}`} size="small" sx={{ mt: 1, px: 0 }}>
              View shop
            </Button>
          </Box>
        )}

        <Row label="Amount" value={formatCents(order.pay_value)} />
        <Row label="Payment method" value={order.pay_type} />
        <Row label="Created" value={formatDateTime(order.create_time)} />
        {order.pay_time && <Row label="Paid" value={formatDateTime(order.pay_time)} />}
        {order.use_time && <Row label="Redeemed" value={formatDateTime(order.use_time)} />}
        {order.refund_time && <Row label="Refunded" value={formatDateTime(order.refund_time)} />}

        <Divider sx={{ my: 2 }} />

        <PageQrCode
          caption={order.status === 'paid' ? 'Show this at the shop to redeem' : 'Scan to open this order'}
        />

        {order.status === 'unpaid' && (
          <>
            {expiresIn > 0 ? (
              <Alert severity="warning" sx={{ mb: 2 }}>
                Pay within <strong>{formatDuration(expiresIn)}</strong> or the order is cancelled and the voucher goes back on sale.
              </Alert>
            ) : (
              <Alert severity="info" sx={{ mb: 2 }}>
                The payment window has passed; the order will be cancelled on the next expiry sweep.
              </Alert>
            )}
            <Button variant="contained" size="large" fullWidth onClick={() => setPaymentOpen(true)}>
              Pay {formatCents(order.pay_value)}
            </Button>
          </>
        )}

        {order.status === 'paid' && (
          <Alert icon={<CheckCircleIcon />} severity="success">
            Paid. Show this order at the shop to redeem your voucher.
          </Alert>
        )}
        {order.status === 'cancelled' && (
          <Alert severity="info">This order was cancelled because it was not paid in time.</Alert>
        )}

        <Button component={Link} to="/orders" sx={{ mt: 2 }}>
          Back to my orders
        </Button>

        <PaymentDialog
          open={isPaymentOpen}
          orderId={order.id}
          amount={order.pay_value}
          onClose={() => setPaymentOpen(false)}
          onPaid={setOrder}
        />
      </CardContent>
    </Card>
  );
};

export default OrderDetailPage;
