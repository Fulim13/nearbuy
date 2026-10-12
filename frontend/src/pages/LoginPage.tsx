import { zodResolver } from '@hookform/resolvers/zod';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { Navigate, useLocation, useNavigate } from 'react-router-dom';
import { z } from 'zod';
import type { User } from '../entities/User';
import apiClient from '../services/api-client';
import { getErrorMessage } from '../services/errors';
import useAuthStore from '../stores/authStore';

// Same rule as the gateway's phoneRe: Malaysian mobile, local format.
const phoneRegex = /^01(?:1\d{8}|[02-46-9]\d{7})$/;

const schema = z.object({
  phone: z.string().regex(phoneRegex, 'Enter a Malaysian mobile number, e.g. 0123456789.'),
  code: z.string().regex(/^\d{6}$/, 'The code is 6 digits.'),
});

type FormData = z.infer<typeof schema>;

interface SendCodeResponse {
  sent: boolean;
  code?: string; // only when the gateway runs with DEV_ECHO_SMS=true
}

interface LoginResponse {
  token: string;
  user: User;
}

const SEND_COOLDOWN = 60; // the gateway allows one code per phone per minute

const LoginPage = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, login } = useAuthStore();
  const [error, setError] = useState('');
  const [devCode, setDevCode] = useState('');
  const [isCodeSent, setCodeSent] = useState(false);
  const [isSending, setSending] = useState(false);
  const [cooldown, setCooldown] = useState(0);

  const {
    register,
    handleSubmit,
    trigger,
    getValues,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: { phone: '', code: '' },
  });

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setTimeout(() => setCooldown(cooldown - 1), 1000);
    return () => clearTimeout(timer);
  }, [cooldown]);

  const from = (location.state as { from?: string } | null)?.from ?? '/';
  if (user) return <Navigate to={from} replace />;

  const sendCode = async () => {
    if (!(await trigger('phone'))) return;
    setError('');
    setSending(true);
    apiClient
      .post<SendCodeResponse>('/user/code', { phone: getValues('phone') })
      .then((res) => {
        setCodeSent(true);
        setCooldown(SEND_COOLDOWN);
        if (res.data.code) {
          setDevCode(res.data.code);
          setValue('code', res.data.code, { shouldValidate: true });
        }
      })
      .catch((err) => setError(getErrorMessage(err)))
      .finally(() => setSending(false));
  };

  const onSubmit = (data: FormData) => {
    setError('');
    return apiClient
      .post<LoginResponse>('/user/login', data)
      .then((res) => {
        login(res.data.token, res.data.user);
        navigate(from, { replace: true });
      })
      .catch((err) => setError(getErrorMessage(err)));
  };

  return (
    <Box sx={{ display: 'flex', justifyContent: 'center', mt: 4 }}>
      <Card sx={{ width: '100%', maxWidth: 420 }}>
        <CardContent sx={{ p: 4 }}>
          <Typography variant="h5">Log in or sign up</Typography>
          <Typography color="text.secondary" sx={{ mb: 3 }}>
            We'll text you a 6-digit code. New numbers get an account automatically.
          </Typography>

          {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
          {devCode && (
            <Alert severity="info" sx={{ mb: 2 }}>
              Dev mode: your code is <strong>{devCode}</strong> (filled in for you).
            </Alert>
          )}

          <form onSubmit={handleSubmit(onSubmit)}>
            <Stack spacing={2}>
              <Stack direction="row" spacing={1} sx={{ alignItems: 'flex-start' }}>
                <TextField
                  {...register('phone')}
                  label="Phone number"
                  placeholder="0123456789"
                  fullWidth
                  autoFocus
                  autoComplete="tel"
                  error={!!errors.phone}
                  helperText={errors.phone?.message}
                  // An OTP form has no password; keep password managers away.
                  slotProps={{ htmlInput: { 'data-1p-ignore': true, 'data-lpignore': 'true' } }}
                />
                <Button
                  variant="outlined"
                  onClick={sendCode}
                  loading={isSending}
                  disabled={cooldown > 0}
                  sx={{ height: 56, whiteSpace: 'nowrap', minWidth: 110 }}
                >
                  {cooldown > 0 ? `Resend ${cooldown}s` : isCodeSent ? 'Resend' : 'Send code'}
                </Button>
              </Stack>

              <TextField
                {...register('code')}
                label="Verification code"
                fullWidth
                disabled={!isCodeSent}
                error={!!errors.code}
                helperText={errors.code?.message ?? 'Codes expire after 2 minutes.'}
                slotProps={{
                  htmlInput: {
                    inputMode: 'numeric',
                    maxLength: 6,
                    autoComplete: 'one-time-code',
                    'data-1p-ignore': true,
                  },
                  // setValue() fills the input without MUI noticing; keep the
                  // label out of the way once a code may be present.
                  inputLabel: { shrink: isCodeSent || undefined },
                }}
              />

              <Button type="submit" variant="contained" size="large" disabled={!isCodeSent} loading={isSubmitting}>
                Log in
              </Button>
            </Stack>
          </form>

          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 3 }}>
            Test users: Ahmad Faizal 0123456789 and Tan Mei Ling 0198765432 (RM 1,000 balance), Ravi Kumar 01123456780 (RM 0 balance). Admin: 0100000000.
          </Typography>
        </CardContent>
      </Card>
    </Box>
  );
};

export default LoginPage;
