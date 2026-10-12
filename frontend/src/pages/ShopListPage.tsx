import AddIcon from '@mui/icons-material/Add';
import {
  Alert,
  Box,
  Button,
  Chip,
  Grid,
  Pagination,
  Stack,
  Typography,
} from '@mui/material';
import { Link, useSearchParams } from 'react-router-dom';
import ShopCard from '../components/ShopCard';
import ShopCardSkeleton from '../components/ShopCardSkeleton';
import useShops from '../hooks/useShops';
import useShopTypes from '../hooks/useShopTypes';

const PAGE_SIZE = 8;

const ShopListPage = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const typeId = Number(searchParams.get('type_id')) || undefined;
  const name = searchParams.get('name') ?? undefined;
  const page = Number(searchParams.get('page')) || 1;

  const { data: types } = useShopTypes();
  const { data, error, isLoading } = useShops({ typeId, name, page, pageSize: PAGE_SIZE });

  const typeName = (id: number) => types?.find((t) => t.id === id)?.name;
  // Name search returns no total (shop-service does not count it), so it
  // pages with Prev/Next: a full page means there may be another one.
  const isNameSearch = !!name && !typeId;
  const pageCount = data && !isNameSearch ? Math.ceil(data.total / PAGE_SIZE) : 0;
  const hasNextPage = !!data && data.shops.length === PAGE_SIZE;

  const selectType = (id: number) => setSearchParams({ type_id: String(id) });

  const changePage = (newPage: number) => {
    const next = new URLSearchParams(searchParams);
    next.set('page', String(newPage));
    setSearchParams(next);
  };

  const heading = name ? `Results for "${name}"` : typeId ? typeName(typeId) ?? 'Shops' : 'Shops';

  return (
    <>
      <Stack direction="row" sx={{ alignItems: 'center', justifyContent: 'space-between', mb: 2, flexWrap: 'wrap', gap: 1 }}>
        <Box>
          <Typography variant="h4">{heading}</Typography>
          {data && (
            <Typography color="text.secondary">
              {isNameSearch
                ? `${data.shops.length} result${data.shops.length === 1 ? '' : 's'} on page ${page}`
                : `${data.total} shop${data.total === 1 ? '' : 's'}`}
            </Typography>
          )}
        </Box>
        <Button component={Link} to="/shops/new" variant="outlined" startIcon={<AddIcon />}>
          Open a shop
        </Button>
      </Stack>

      <Stack direction="row" spacing={1} sx={{ mb: 3, flexWrap: 'wrap', rowGap: 1 }}>
        {types?.map((t) => (
          <Chip
            key={t.id}
            label={t.name}
            clickable
            color={t.id === typeId ? 'primary' : 'default'}
            variant={t.id === typeId ? 'filled' : 'outlined'}
            onClick={() => selectType(t.id)}
          />
        ))}
      </Stack>

      {!typeId && !name && (
        <Alert severity="info">Pick a category or search for a shop by name.</Alert>
      )}
      {error && <Alert severity="error">{error}</Alert>}
      {data?.shops.length === 0 && <Alert severity="info">No shops found.</Alert>}

      <Grid container spacing={2}>
        {isLoading &&
          Array.from({ length: PAGE_SIZE }).map((_, i) => (
            <Grid key={i} size={{ xs: 12, sm: 6, md: 3 }}>
              <ShopCardSkeleton />
            </Grid>
          ))}
        {!isLoading &&
          data?.shops.map((shop) => (
            <Grid key={shop.id} size={{ xs: 12, sm: 6, md: 3 }}>
              <ShopCard shop={shop} typeName={typeName(shop.type_id)} />
            </Grid>
          ))}
      </Grid>

      {pageCount > 1 && (
        <Box sx={{ display: 'flex', justifyContent: 'center', mt: 4 }}>
          <Pagination
            count={pageCount}
            page={page}
            color="primary"
            onChange={(_, value) => changePage(value)}
          />
        </Box>
      )}
      {isNameSearch && (page > 1 || hasNextPage) && (
        <Stack direction="row" spacing={2} sx={{ justifyContent: 'center', mt: 4 }}>
          <Button disabled={page <= 1} onClick={() => changePage(page - 1)}>Previous</Button>
          <Button disabled={!hasNextPage} onClick={() => changePage(page + 1)}>Next</Button>
        </Stack>
      )}
    </>
  );
};

export default ShopListPage;
