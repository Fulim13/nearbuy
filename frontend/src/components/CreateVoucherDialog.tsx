import { zodResolver } from '@hookform/resolvers/zod';
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Grid,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
} from '@mui/material';
import { useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { z } from 'zod';
import type { Voucher } from '../entities/Voucher';
import apiClient from '../services/api-client';
import { getErrorMessage } from '../services/errors';
import { localInputToRfc3339, toLocalInput } from '../utils/datetime';

const schema = z
  .object({
    type: z.enum(['normal', 'seckill']),
    title: z.string().min(1, 'Title is required.').max(255),
    subTitle: z.string().max(255),
    rules: z.string().max(1024),
    payValue: z.number({ error: 'Enter a price.' }).min(0, 'Cannot be negative.'),
    actualValue: z.number({ error: 'Enter a value.' }).min(0, 'Cannot be negative.'),
    stock: z.number({ error: 'Enter the stock.' }).int().min(1, 'At least 1.'),
    beginTime: z.string(),
    endTime: z.string(),
  })
  .refine((data) => data.type === 'normal' || data.endTime > data.beginTime, {
    message: 'End time must be after the start time.',
    path: ['endTime'],
  });

type FormData = z.infer<typeof schema>;

interface Props {
  shopId: number;
  open: boolean;
  onClose: () => void;
  onCreated: (voucher: Voucher) => void;
}

const CreateVoucherDialog = ({ shopId, open, onClose, onCreated }: Props) => {
  const [error, setError] = useState('');
  const now = new Date();

  const {
    register,
    handleSubmit,
    control,
    watch,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: {
      type: 'seckill',
      title: '',
      subTitle: '',
      rules: '',
      payValue: 40,
      actualValue: 50,
      stock: 100,
      // Default to a sale that opens in one minute, so the countdown is visible.
      beginTime: toLocalInput(new Date(now.getTime() + 60_000)),
      endTime: toLocalInput(new Date(now.getTime() + 60 * 60_000)),
    },
  });

  const type = watch('type');

  const onSubmit = (data: FormData) => {
    setError('');
    const base = {
      shop_id: shopId,
      title: data.title,
      sub_title: data.subTitle,
      rules: data.rules,
      pay_value: Math.round(data.payValue * 100),
      actual_value: Math.round(data.actualValue * 100),
    };

    const request =
      data.type === 'seckill'
        ? apiClient.post<Voucher>('/vouchers/seckill', {
            ...base,
            stock: data.stock,
            begin_time: localInputToRfc3339(data.beginTime),
            end_time: localInputToRfc3339(data.endTime),
          })
        : apiClient.post<Voucher>('/vouchers', base);

    return request
      .then((res) => {
        onCreated(res.data);
        reset();
        onClose();
      })
      .catch((err) => setError(getErrorMessage(err)));
  };

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <form onSubmit={handleSubmit(onSubmit)}>
        <DialogTitle>Add a voucher</DialogTitle>
        <DialogContent>
          {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
          <Controller
            name="type"
            control={control}
            render={({ field }) => (
              <ToggleButtonGroup
                exclusive
                fullWidth
                color="primary"
                value={field.value}
                onChange={(_, value) => value && field.onChange(value)}
                sx={{ mb: 2, mt: 1 }}
              >
                <ToggleButton value="seckill">Flash sale (seckill)</ToggleButton>
                <ToggleButton value="normal">Regular voucher</ToggleButton>
              </ToggleButtonGroup>
            )}
          />

          <Grid container spacing={2}>
            <Grid size={12}>
              <TextField
                {...register('title')}
                label="Title"
                fullWidth
                error={!!errors.title}
                helperText={errors.title?.message}
              />
            </Grid>
            <Grid size={12}>
              <TextField {...register('subTitle')} label="Subtitle" fullWidth />
            </Grid>
            <Grid size={12}>
              <TextField {...register('rules')} label="Rules" fullWidth multiline minRows={2} />
            </Grid>
            <Grid size={6}>
              <TextField
                {...register('payValue', { valueAsNumber: true })}
                label="Price (RM)"
                type="number"
                fullWidth
                slotProps={{ htmlInput: { step: '0.01' } }}
                error={!!errors.payValue}
                helperText={errors.payValue?.message ?? 'What the customer pays'}
              />
            </Grid>
            <Grid size={6}>
              <TextField
                {...register('actualValue', { valueAsNumber: true })}
                label="Face value (RM)"
                type="number"
                fullWidth
                slotProps={{ htmlInput: { step: '0.01' } }}
                error={!!errors.actualValue}
                helperText={errors.actualValue?.message ?? 'What it is worth in store'}
              />
            </Grid>

            {type === 'seckill' && (
              <>
                <Grid size={12}>
                  <TextField
                    {...register('stock', { valueAsNumber: true })}
                    label="Stock"
                    type="number"
                    fullWidth
                    error={!!errors.stock}
                    helperText={errors.stock?.message}
                  />
                </Grid>
                <Grid size={6}>
                  <TextField
                    {...register('beginTime')}
                    label="Sale starts"
                    type="datetime-local"
                    fullWidth
                    slotProps={{ inputLabel: { shrink: true } }}
                  />
                </Grid>
                <Grid size={6}>
                  <TextField
                    {...register('endTime')}
                    label="Sale ends"
                    type="datetime-local"
                    fullWidth
                    slotProps={{ inputLabel: { shrink: true } }}
                    error={!!errors.endTime}
                    helperText={errors.endTime?.message}
                  />
                </Grid>
              </>
            )}
          </Grid>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="contained" loading={isSubmitting}>
            Create voucher
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
};

export default CreateVoucherDialog;
