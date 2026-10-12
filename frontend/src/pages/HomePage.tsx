import {
  Alert,
  Box,
  Card,
  CardActionArea,
  Grid,
  Skeleton,
  Typography,
} from '@mui/material';
import { Link } from 'react-router-dom';
import ShopTypeIcon from '../components/ShopTypeIcon';
import useShopTypes from '../hooks/useShopTypes';
import useAuthStore from '../stores/authStore';

const HomePage = () => {
  const { data: types, error, isLoading } = useShopTypes();
  const user = useAuthStore((s) => s.user);

  return (
    <>
      <Box sx={{ mb: 4 }}>
        <Typography variant="h4">
          {user ? `Hi ${user.nickname}, what's nearby today?` : "Discover what's nearby"}
        </Typography>
        <Typography color="text.secondary" sx={{ mt: 1 }}>
          Browse local shops, grab flash-sale vouchers before they run out, and pay with your balance.
        </Typography>
      </Box>

      <Typography variant="h6" sx={{ mb: 2 }}>
        Categories
      </Typography>

      {error && <Alert severity="error">{error}</Alert>}

      <Grid container spacing={2}>
        {isLoading &&
          Array.from({ length: 10 }).map((_, i) => (
            <Grid key={i} size={{ xs: 6, sm: 4, md: 2.4 }}>
              <Skeleton variant="rounded" height={110} />
            </Grid>
          ))}
        {types?.map((type) => (
          <Grid key={type.id} size={{ xs: 6, sm: 4, md: 2.4 }}>
            <Card>
              <CardActionArea
                component={Link}
                to={`/shops?type_id=${type.id}`}
                sx={{ p: 2.5, display: 'flex', flexDirection: 'column', gap: 1 }}
              >
                <Box
                  sx={{
                    width: 52,
                    height: 52,
                    borderRadius: '50%',
                    bgcolor: '#FFE8CC',
                    color: 'primary.main',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                  }}
                >
                  <ShopTypeIcon name={type.name} />
                </Box>
                <Typography sx={{ fontWeight: 600 }}>{type.name}</Typography>
              </CardActionArea>
            </Card>
          </Grid>
        ))}
      </Grid>
    </>
  );
};

export default HomePage;
