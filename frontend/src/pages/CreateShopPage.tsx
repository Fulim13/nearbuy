import { Card, CardContent, Typography } from '@mui/material';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import ShopForm, { type ShopFormData } from '../components/ShopForm';
import type { Shop } from '../entities/Shop';
import apiClient from '../services/api-client';
import { getErrorMessage } from '../services/errors';

const CreateShopPage = () => {
  const navigate = useNavigate();
  const [error, setError] = useState('');

  const onSubmit = (data: ShopFormData) => {
    setError('');
    return apiClient
      .post<Shop>('/shops', {
        name: data.name,
        type_id: data.typeId,
        images: data.images,
        area: data.area,
        address: data.address,
        x: data.x,
        y: data.y,
        avg_price: data.avgPrice,
        open_hours: data.openHours,
      })
      .then((res) => navigate(`/shops/${res.data.id}`))
      .catch((err) => setError(getErrorMessage(err)));
  };

  return (
    <Card sx={{ maxWidth: 760, mx: 'auto' }}>
      <CardContent sx={{ p: 4 }}>
        <Typography variant="h5" sx={{ mb: 3 }}>
          Open a shop
        </Typography>
        <ShopForm submitLabel="Create shop" error={error} onSubmit={onSubmit} />
      </CardContent>
    </Card>
  );
};

export default CreateShopPage;
