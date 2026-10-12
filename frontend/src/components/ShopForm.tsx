import { zodResolver } from '@hookform/resolvers/zod';
import { Alert, Button, Grid, MenuItem, TextField } from '@mui/material';
import { Controller, useForm } from 'react-hook-form';
import { z } from 'zod';
import useShopTypes from '../hooks/useShopTypes';

const schema = z.object({
  name: z.string().min(1, 'Name is required.').max(128),
  typeId: z.number({ error: 'Pick a category.' }).int().positive('Pick a category.'),
  area: z.string().max(128),
  address: z.string().min(1, 'Address is required.').max(255),
  x: z.number({ error: 'Enter a longitude.' }).min(-180).max(180),
  y: z.number({ error: 'Enter a latitude.' }).min(-85.05).max(85.05),
  avgPrice: z.number({ error: 'Enter a price.' }).int().min(0),
  openHours: z.string().max(32),
  images: z.string().max(2048),
});

export type ShopFormData = z.infer<typeof schema>;

interface Props {
  defaultValues?: Partial<ShopFormData>;
  // Location is fixed after creation: PATCH /shops/:id does not accept x/y.
  isEdit?: boolean;
  submitLabel: string;
  error?: string;
  onSubmit: (data: ShopFormData) => Promise<unknown>;
}

const ShopForm = ({ defaultValues, isEdit, submitLabel, error, onSubmit }: Props) => {
  const { data: types } = useShopTypes();

  const {
    register,
    handleSubmit,
    control,
    formState: { errors, isSubmitting },
  } = useForm<ShopFormData>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: '',
      typeId: 1,
      area: '',
      address: '',
      // Kuala Lumpur city centre.
      x: 101.6869,
      y: 3.139,
      avgPrice: 30,
      openHours: '10:00-22:00',
      images: '',
      ...defaultValues,
    },
  });

  return (
    <form onSubmit={handleSubmit(onSubmit)}>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, sm: 8 }}>
          <TextField {...register('name')} label="Shop name" fullWidth error={!!errors.name} helperText={errors.name?.message} />
        </Grid>
        <Grid size={{ xs: 12, sm: 4 }}>
          <Controller
            name="typeId"
            control={control}
            render={({ field }) => (
              <TextField
                select
                label="Category"
                fullWidth
                value={types ? field.value : ''}
                onChange={(e) => field.onChange(Number(e.target.value))}
                error={!!errors.typeId}
                helperText={errors.typeId?.message}
              >
                {types?.map((t) => (
                  <MenuItem key={t.id} value={t.id}>
                    {t.name}
                  </MenuItem>
                ))}
              </TextField>
            )}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 4 }}>
          <TextField {...register('area')} label="Area" fullWidth placeholder="e.g. Bukit Bintang" />
        </Grid>
        <Grid size={{ xs: 12, sm: 8 }}>
          <TextField {...register('address')} label="Address" fullWidth error={!!errors.address} helperText={errors.address?.message} />
        </Grid>
        {!isEdit && (
          <>
            <Grid size={6}>
              <TextField
                {...register('x', { valueAsNumber: true })}
                label="Longitude (x)"
                type="number"
                fullWidth
                slotProps={{ htmlInput: { step: 'any' } }}
                error={!!errors.x}
                helperText={errors.x?.message}
              />
            </Grid>
            <Grid size={6}>
              <TextField
                {...register('y', { valueAsNumber: true })}
                label="Latitude (y)"
                type="number"
                fullWidth
                slotProps={{ htmlInput: { step: 'any' } }}
                error={!!errors.y}
                helperText={errors.y?.message}
              />
            </Grid>
          </>
        )}
        <Grid size={{ xs: 12, sm: 6 }}>
          <TextField
            {...register('avgPrice', { valueAsNumber: true })}
            label="Average price per person (RM)"
            type="number"
            fullWidth
            error={!!errors.avgPrice}
            helperText={errors.avgPrice?.message}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6 }}>
          <TextField {...register('openHours')} label="Opening hours" fullWidth placeholder="10:00-22:00" />
        </Grid>
        <Grid size={12}>
          <TextField {...register('images')} label="Image URL(s)" fullWidth helperText="Optional, comma separated" />
        </Grid>
        <Grid size={12}>
          <Button type="submit" variant="contained" size="large" loading={isSubmitting}>
            {submitLabel}
          </Button>
        </Grid>
      </Grid>
    </form>
  );
};

export default ShopForm;
