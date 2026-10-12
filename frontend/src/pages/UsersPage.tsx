import { zodResolver } from '@hookform/resolvers/zod';
import { Button, Card, CardContent, Chip, Stack, TextField, Typography } from '@mui/material';
import { useForm } from 'react-hook-form';
import { useNavigate } from 'react-router-dom';
import { z } from 'zod';

const schema = z.object({
  id: z.number({ error: 'Enter a user id.' }).int().positive('Enter a user id.'),
});

type FormData = z.infer<typeof schema>;

// The API has no user search, so people are found by id.
const UsersPage = () => {
  const navigate = useNavigate();
  const {
    register,
    handleSubmit,
    formState: { errors, isValid },
  } = useForm<FormData>({ resolver: zodResolver(schema), mode: 'onChange' });

  const onSubmit = (data: FormData) => navigate(`/users/${data.id}`);

  return (
    <Card sx={{ maxWidth: 520, mx: 'auto' }}>
      <CardContent sx={{ p: 4 }}>
        <Typography variant="h5">Find people</Typography>
        <Typography color="text.secondary" sx={{ mb: 3 }}>
          Look someone up by their user id to follow them and see who you both follow.
        </Typography>
        <form onSubmit={handleSubmit(onSubmit)}>
          <Stack direction="row" spacing={1} sx={{ alignItems: 'flex-start' }}>
            <TextField
              {...register('id', { valueAsNumber: true })}
              label="User id"
              type="number"
              fullWidth
              error={!!errors.id}
              helperText={errors.id?.message}
            />
            <Button type="submit" variant="contained" disabled={!isValid} sx={{ height: 56 }}>
              Go
            </Button>
          </Stack>
        </form>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 3, mb: 1 }}>
          Seeded users
        </Typography>
        <Stack direction="row" spacing={1}>
          {[1, 2, 3].map((id) => (
            <Chip key={id} label={`User #${id}`} clickable onClick={() => navigate(`/users/${id}`)} />
          ))}
        </Stack>
      </CardContent>
    </Card>
  );
};

export default UsersPage;
