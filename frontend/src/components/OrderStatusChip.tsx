import { Chip, type ChipProps } from '@mui/material';
import type { OrderStatus } from '../entities/Order';

const colorMap: Record<OrderStatus, ChipProps['color']> = {
  unpaid: 'warning',
  paid: 'success',
  redeemed: 'info',
  cancelled: 'default',
  refunding: 'secondary',
  refunded: 'default',
};

interface Props {
  status: OrderStatus;
}

const OrderStatusChip = ({ status }: Props) => {
  return (
    <Chip
      size="small"
      label={status.toUpperCase()}
      color={colorMap[status]}
      sx={{ fontWeight: 700, letterSpacing: '0.04em' }}
    />
  );
};

export default OrderStatusChip;
