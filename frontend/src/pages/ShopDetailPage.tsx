import AccessTimeIcon from '@mui/icons-material/AccessTime';
import AddIcon from '@mui/icons-material/Add';
import EditIcon from '@mui/icons-material/Edit';
import LocationOnOutlinedIcon from '@mui/icons-material/LocationOnOutlined';
import StarIcon from '@mui/icons-material/Star';
import {
  Alert,
  Box,
  Breadcrumbs,
  Button,
  Card,
  Chip,
  Grid,
  Link as MuiLink,
  Skeleton,
  Stack,
  Typography,
} from '@mui/material';
import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import CreateVoucherDialog from '../components/CreateVoucherDialog';
import ShopCover from '../components/ShopCover';
import VoucherCard from '../components/VoucherCard';
import useShop from '../hooks/useShop';
import useShopTypes from '../hooks/useShopTypes';
import useVouchers from '../hooks/useVouchers';
import { useIsAdmin } from '../stores/authStore';
import { formatRinggit } from '../utils/money';

const Stat = ({ label, value }: { label: string; value: string }) => (
  <Box>
    <Typography variant="h6">{value}</Typography>
    <Typography variant="caption" color="text.secondary">
      {label}
    </Typography>
  </Box>
);

const ShopDetailPage = () => {
  const { id } = useParams();
  const isAdmin = useIsAdmin();
  const [isDialogOpen, setDialogOpen] = useState(false);

  const { data: shop, error, isLoading } = useShop(id);
  const { data: types } = useShopTypes();
  const { data: vouchers, setData: setVouchers, error: voucherError } = useVouchers(id);

  const typeName = types?.find((t) => t.id === shop?.type_id)?.name;

  if (isLoading || (!shop && !error))
    return <Skeleton variant="rounded" height={320} />;
  if (error || !shop) return <Alert severity="error">{error}</Alert>;

  return (
    <>
      <Breadcrumbs sx={{ mb: 2 }}>
        <MuiLink component={Link} to="/" underline="hover" color="inherit">
          Explore
        </MuiLink>
        <MuiLink component={Link} to={`/?type_id=${shop.type_id}`} underline="hover" color="inherit">
          {typeName ?? 'Shops'}
        </MuiLink>
        <Typography color="text.primary">{shop.name}</Typography>
      </Breadcrumbs>

      <Card sx={{ mb: 4 }}>
        <ShopCover id={shop.id} typeName={typeName} images={shop.images} height={180} />
        <Box sx={{ p: 3 }}>
          <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: 2 }}>
            <Box>
              <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
                <Typography variant="h4">{shop.name}</Typography>
                {typeName && <Chip label={typeName} size="small" />}
              </Stack>
              <Stack direction="row" spacing={2} sx={{ mt: 1, color: 'text.secondary', flexWrap: 'wrap' }}>
                <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center' }}>
                  <LocationOnOutlinedIcon fontSize="small" />
                  <Typography variant="body2">
                    {[shop.address, shop.area].filter(Boolean).join(', ')}
                  </Typography>
                </Stack>
                {shop.open_hours && (
                  <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center' }}>
                    <AccessTimeIcon fontSize="small" />
                    <Typography variant="body2">{shop.open_hours}</Typography>
                  </Stack>
                )}
              </Stack>
              <Typography variant="caption" color="text.secondary">
                ({shop.y.toFixed(5)}, {shop.x.toFixed(5)})
              </Typography>
            </Box>
            {isAdmin && (
              <Button component={Link} to={`/shops/${shop.id}/edit`} startIcon={<EditIcon />} variant="outlined">
                Edit shop
              </Button>
            )}
          </Stack>

          <Grid container spacing={3} sx={{ mt: 1 }}>
            <Grid size="auto">
              <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center' }}>
                <StarIcon sx={{ color: '#F59F00' }} />
                <Stat label="Score" value={(shop.score / 10).toFixed(1)} />
              </Stack>
            </Grid>
            <Grid size="auto">
              <Stat label="Reviews" value={shop.comments.toLocaleString()} />
            </Grid>
            <Grid size="auto">
              <Stat label="Vouchers sold" value={shop.sold.toLocaleString()} />
            </Grid>
            <Grid size="auto">
              <Stat label="Avg per person" value={formatRinggit(shop.avg_price)} />
            </Grid>
          </Grid>
        </Box>
      </Card>

      <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Typography variant="h5">Vouchers</Typography>
        {isAdmin && (
          <Button variant="contained" startIcon={<AddIcon />} onClick={() => setDialogOpen(true)}>
            Add voucher
          </Button>
        )}
      </Stack>

      {voucherError && <Alert severity="error">{voucherError}</Alert>}
      {vouchers?.length === 0 && (
        <Alert severity="info">
          {isAdmin
            ? 'This shop has no vouchers yet. Add a flash sale to try the seckill flow.'
            : 'This shop has no vouchers yet. Check back soon.'}
        </Alert>
      )}
      <Stack spacing={2}>
        {vouchers?.map((voucher) => (
          <VoucherCard key={voucher.id} voucher={voucher} />
        ))}
      </Stack>

      {isAdmin && (
        <CreateVoucherDialog
          shopId={shop.id}
          open={isDialogOpen}
          onClose={() => setDialogOpen(false)}
          onCreated={(voucher) => setVouchers([...(vouchers ?? []), voucher])}
        />
      )}
    </>
  );
};

export default ShopDetailPage;
