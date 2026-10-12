import BoltIcon from '@mui/icons-material/Bolt';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Stack,
  Typography,
} from '@mui/material';
import { useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import type { SeckillResult } from '../entities/Order';
import type { Voucher } from '../entities/Voucher';
import useNow from '../hooks/useNow';
import apiClient from '../services/api-client';
import { getErrorMessage } from '../services/errors';
import useAuthStore from '../stores/authStore';
import { formatCents } from '../utils/money';
import { formatDuration, getVoucherState } from '../utils/voucherState';

interface Props {
  voucher: Voucher;
}

const VoucherCard = ({ voucher }: Props) => {
  const now = useNow();
  const user = useAuthStore((s) => s.user);
  const navigate = useNavigate();
  const location = useLocation();
  const [isGrabbing, setGrabbing] = useState(false);
  const [error, setError] = useState('');

  const state = getVoucherState(voucher, now);
  const discount = voucher.actual_value - voucher.pay_value;

  const handleGrab = () => {
    if (!user) {
      navigate('/login', { state: { from: location.pathname } });
      return;
    }
    setGrabbing(true);
    setError('');
    apiClient
      .post<SeckillResult>(`/voucher-order/seckill/${voucher.id}`)
      .then((res) => navigate(`/orders/${res.data.order_id}`))
      .catch((err) => {
        setError(getErrorMessage(err));
        setGrabbing(false);
      });
  };

  return (
    <Card sx={{ display: 'flex', overflow: 'hidden' }}>
      <Box
        sx={{
          width: 120,
          flexShrink: 0,
          bgcolor: voucher.type === 'seckill' ? 'primary.main' : 'secondary.main',
          color: 'white',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          p: 1,
          borderRight: '2px dashed rgba(255,255,255,0.6)',
        }}
      >
        <Typography variant="h6" sx={{ lineHeight: 1.1 }}>
          {formatCents(voucher.pay_value)}
        </Typography>
        <Typography variant="caption" sx={{ textDecoration: 'line-through', opacity: 0.8 }}>
          {formatCents(voucher.actual_value)}
        </Typography>
      </Box>

      <CardContent sx={{ flexGrow: 1 }}>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center', mb: 0.5, flexWrap: 'wrap' }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>
            {voucher.title}
          </Typography>
          {voucher.type === 'seckill' && (
            <Chip icon={<BoltIcon />} label="Flash sale" size="small" color="primary" variant="outlined" />
          )}
          {discount > 0 && (
            <Chip label={`Save ${formatCents(discount)}`} size="small" color="success" variant="outlined" />
          )}
        </Stack>
        {voucher.sub_title && (
          <Typography variant="body2" color="text.secondary">
            {voucher.sub_title}
          </Typography>
        )}
        {voucher.rules && (
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.5 }}>
            Rules: {voucher.rules}
          </Typography>
        )}

        <Stack direction="row" spacing={2} sx={{ alignItems: 'center', mt: 1.5, flexWrap: 'wrap' }}>
          {state.kind === 'in-store' && (
            <Typography variant="body2" color="text.secondary">
              Regular voucher · redeem in store
            </Typography>
          )}
          {state.kind === 'upcoming' && (
            <>
              <Button variant="outlined" disabled>
                Starts in {formatDuration(state.msLeft)}
              </Button>
              <Typography variant="body2" color="text.secondary">
                {voucher.stock} in stock
              </Typography>
            </>
          )}
          {state.kind === 'live' && (
            <>
              <Button variant="contained" onClick={handleGrab} loading={isGrabbing} startIcon={<BoltIcon />}>
                Grab now
              </Button>
              <Typography variant="body2" color="text.secondary">
                {voucher.stock} left · ends in {formatDuration(state.msLeft)}
              </Typography>
            </>
          )}
          {state.kind === 'sold-out' && <Button disabled variant="outlined">Sold out</Button>}
          {state.kind === 'ended' && <Button disabled variant="outlined">Sale ended</Button>}
        </Stack>

        {error && (
          <Alert severity="warning" sx={{ mt: 1.5 }} onClose={() => setError('')}>
            {error}
          </Alert>
        )}
      </CardContent>
    </Card>
  );
};

export default VoucherCard;
