import { Alert, Card, CardContent, Skeleton, Typography } from '@mui/material';
import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import ShopForm, { type ShopFormData } from '../components/ShopForm';
import useShop from '../hooks/useShop';
import apiClient from '../services/api-client';
import { getErrorMessage } from '../services/errors';

const EditShopPage = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const { data: shop, error: loadError } = useShop(id);
  const [error, setError] = useState('');

  const onSubmit = (data: ShopFormData) => {
    setError('');
    // PATCH: the gateway treats absent fields as "leave unchanged".
    return apiClient
      .patch(`/shops/${id}`, {
        name: data.name,
        type_id: data.typeId,
        images: data.images,
        area: data.area,
        address: data.address,
        avg_price: data.avgPrice,
        open_hours: data.openHours,
      })
      .then(() => navigate(`/shops/${id}`))
      .catch((err) => setError(getErrorMessage(err)));
  };

  if (loadError) return <Alert severity="error">{loadError}</Alert>;
  if (!shop) return <Skeleton variant="rounded" height={400} />;

  return (
    <Card sx={{ maxWidth: 760, mx: 'auto' }}>
      <CardContent sx={{ p: 4 }}>
        <Typography variant="h5" sx={{ mb: 3 }}>
          Edit {shop.name}
        </Typography>
        <ShopForm
          isEdit
          submitLabel="Save changes"
          error={error}
          onSubmit={onSubmit}
          defaultValues={{
            name: shop.name,
            typeId: shop.type_id,
            area: shop.area,
            address: shop.address,
            x: shop.x,
            y: shop.y,
            avgPrice: shop.avg_price,
            openHours: shop.open_hours,
            images: shop.images,
          }}
        />
      </CardContent>
    </Card>
  );
};

export default EditShopPage;
