import AccountBalanceIcon from '@mui/icons-material/AccountBalance';
import AccountBalanceWalletIcon from '@mui/icons-material/AccountBalanceWallet';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import CreditCardIcon from '@mui/icons-material/CreditCard';
import ErrorIcon from '@mui/icons-material/Error';
import LockOutlinedIcon from '@mui/icons-material/LockOutlined';
import PhoneIphoneIcon from '@mui/icons-material/PhoneIphone';
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  LinearProgress,
  Radio,
  Stack,
  Typography,
  type SvgIconProps,
} from '@mui/material';
import { useEffect, useState, type ComponentType } from 'react';
import type { Order } from '../entities/Order';
import apiClient from '../services/api-client';
import { getErrorMessage, getStatus } from '../services/errors';
import { formatCents } from '../utils/money';

type MethodId = 'balance' | 'card' | 'ewallet' | 'fpx';

interface PaymentMethod {
  id: MethodId;
  label: string;
  description: string;
  icon: ComponentType<SvgIconProps>;
  // Only balance is real: order-service's pay saga always deducts balance.
  enabled: boolean;
}

const methods: PaymentMethod[] = [
  { id: 'balance', label: 'NearBuy Balance', description: 'Pay instantly from your wallet', icon: AccountBalanceWalletIcon, enabled: true },
  { id: 'card', label: 'Credit / Debit Card', description: 'Visa, Mastercard', icon: CreditCardIcon, enabled: false },
  { id: 'ewallet', label: "Touch 'n Go eWallet", description: 'Pay with your TNG app', icon: PhoneIphoneIcon, enabled: false },
  { id: 'fpx', label: 'FPX Online Banking', description: 'Maybank2u, CIMB Clicks and more', icon: AccountBalanceIcon, enabled: false },
];

// Purely cosmetic: the real payment is one request that usually takes well
// under a second. The steps make the wait read like a payment gateway.
const STEPS = [
  'Connecting to payment gateway…',
  'Verifying your balance…',
  'Authorising payment…',
  'Confirming with the shop…',
];
const STEP_MS = 700;

type Stage = 'choose' | 'processing' | 'success' | 'failed';

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

interface Props {
  open: boolean;
  orderId: string;
  amount: number; // cents
  onClose: () => void;
  onPaid: (order: Order) => void;
}

const PaymentDialog = ({ open, orderId, amount, onClose, onPaid }: Props) => {
  const [method, setMethod] = useState<MethodId>('balance');
  const [stage, setStage] = useState<Stage>('choose');
  const [step, setStep] = useState(0);
  const [error, setError] = useState('');
  const [paidOrder, setPaidOrder] = useState<Order | null>(null);

  // Advance the fake progress while processing.
  useEffect(() => {
    if (stage !== 'processing') return;
    const timer = setInterval(
      () => setStep((s) => Math.min(s + 1, STEPS.length - 1)),
      STEP_MS
    );
    return () => clearInterval(timer);
  }, [stage]);

  const handlePay = async () => {
    setStage('processing');
    setStep(0);
    setError('');

    // The real request starts immediately; the animation only sets a floor on
    // how long the dialog shows "processing".
    const [result] = await Promise.allSettled([
      apiClient.post<Order>(`/voucher-order/${orderId}/pay`),
      delay(STEPS.length * STEP_MS),
    ]);

    if (result.status === 'fulfilled') {
      setPaidOrder(result.value.data);
      setStage('success');
    } else {
      setError(
        getStatus(result.reason) === 402
          ? 'Insufficient balance. Nothing was charged and your order is still unpaid.'
          : getErrorMessage(result.reason)
      );
      setStage('failed');
    }
  };

  const handleClose = () => {
    // A payment in flight cannot be abandoned from the UI.
    if (stage === 'processing') return;
    if (stage === 'success' && paidOrder) onPaid(paidOrder);
    setStage('choose');
    onClose();
  };

  const progress = stage === 'processing' ? ((step + 1) / STEPS.length) * 100 : 0;

  return (
    <Dialog open={open} onClose={handleClose} fullWidth maxWidth="xs">
      {stage === 'choose' && (
        <>
          <DialogTitle>Choose payment method</DialogTitle>
          <DialogContent>
            <Stack spacing={1.5}>
              {methods.map((m) => {
                const Icon = m.icon;
                const selected = method === m.id;
                return (
                  <Box
                    key={m.id}
                    onClick={() => m.enabled && setMethod(m.id)}
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 1.5,
                      p: 1.5,
                      borderRadius: 2,
                      border: '2px solid',
                      borderColor: selected ? 'primary.main' : '#EDE6DB',
                      bgcolor: selected ? '#FFF4E6' : 'background.paper',
                      cursor: m.enabled ? 'pointer' : 'not-allowed',
                      opacity: m.enabled ? 1 : 0.55,
                    }}
                  >
                    <Icon color={selected ? 'primary' : 'action'} />
                    <Box sx={{ flexGrow: 1 }}>
                      <Typography sx={{ fontWeight: 600 }}>{m.label}</Typography>
                      <Typography variant="caption" color="text.secondary">
                        {m.description}
                      </Typography>
                    </Box>
                    {m.enabled ? (
                      <Radio checked={selected} size="small" />
                    ) : (
                      <Chip label="Coming soon" size="small" />
                    )}
                  </Box>
                );
              })}
            </Stack>

            <Stack direction="row" sx={{ justifyContent: 'space-between', mt: 3 }}>
              <Typography color="text.secondary">Total</Typography>
              <Typography variant="h6">{formatCents(amount)}</Typography>
            </Stack>
          </DialogContent>
          <DialogActions sx={{ px: 3, pb: 2 }}>
            <Button onClick={handleClose}>Cancel</Button>
            <Button variant="contained" startIcon={<LockOutlinedIcon />} onClick={handlePay}>
              Pay {formatCents(amount)}
            </Button>
          </DialogActions>
        </>
      )}

      {stage === 'processing' && (
        <DialogContent sx={{ py: 5, textAlign: 'center' }}>
          <LockOutlinedIcon color="primary" sx={{ fontSize: 44 }} />
          <Typography variant="h6" sx={{ mt: 1 }}>
            Processing payment
          </Typography>
          <Typography color="text.secondary" sx={{ mt: 0.5, minHeight: 24 }}>
            {STEPS[step]}
          </Typography>
          <LinearProgress variant="determinate" value={progress} sx={{ mt: 3, height: 8, borderRadius: 4 }} />
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 2 }}>
            Please don't close this window.
          </Typography>
        </DialogContent>
      )}

      {stage === 'success' && (
        <>
          <DialogContent sx={{ py: 5, textAlign: 'center' }}>
            <CheckCircleIcon color="success" sx={{ fontSize: 64 }} />
            <Typography variant="h6" sx={{ mt: 1 }}>
              Payment successful
            </Typography>
            <Typography color="text.secondary">
              {formatCents(amount)} paid with NearBuy Balance
            </Typography>
          </DialogContent>
          <DialogActions sx={{ px: 3, pb: 2 }}>
            <Button variant="contained" fullWidth onClick={handleClose}>
              Done
            </Button>
          </DialogActions>
        </>
      )}

      {stage === 'failed' && (
        <>
          <DialogContent sx={{ py: 5, textAlign: 'center' }}>
            <ErrorIcon color="error" sx={{ fontSize: 64 }} />
            <Typography variant="h6" sx={{ mt: 1 }}>
              Payment failed
            </Typography>
            <Typography color="text.secondary">{error}</Typography>
          </DialogContent>
          <DialogActions sx={{ px: 3, pb: 2 }}>
            <Button onClick={handleClose}>Close</Button>
            <Button variant="contained" onClick={() => setStage('choose')}>
              Try again
            </Button>
          </DialogActions>
        </>
      )}
    </Dialog>
  );
};

export default PaymentDialog;
