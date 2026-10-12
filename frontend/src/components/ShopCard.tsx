import LocationOnOutlinedIcon from '@mui/icons-material/LocationOnOutlined';
import StarIcon from '@mui/icons-material/Star';
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Stack,
  Typography,
} from '@mui/material';
import { Link } from 'react-router-dom';
import type { Shop } from '../entities/Shop';
import { formatRinggit } from '../utils/money';
import ShopCover from './ShopCover';

interface Props {
  shop: Shop;
  typeName?: string;
}

const ShopCard = ({ shop, typeName }: Props) => {
  return (
    <Card sx={{ height: '100%' }}>
      <CardActionArea component={Link} to={`/shops/${shop.id}`} sx={{ height: '100%' }}>
        <ShopCover id={shop.id} typeName={typeName} />
        <CardContent>
          <Typography variant="subtitle1" sx={{ fontWeight: 700 }} noWrap>
            {shop.name}
          </Typography>
          <Stack direction="row" spacing={1} sx={{ alignItems: 'center', my: 0.5 }}>
            <StarIcon sx={{ fontSize: 16, color: '#F59F00' }} />
            <Typography variant="body2">{(shop.score / 10).toFixed(1)}</Typography>
            <Typography variant="body2" color="text.secondary">
              · {shop.comments.toLocaleString()} reviews
            </Typography>
          </Stack>
          <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center' }}>
            <LocationOnOutlinedIcon sx={{ fontSize: 16, color: 'text.secondary' }} />
            <Typography variant="body2" color="text.secondary" noWrap>
              {shop.area || 'Unknown area'}
            </Typography>
          </Stack>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', mt: 1 }}>
            <Typography variant="body2" sx={{ fontWeight: 600 }}>
              {formatRinggit(shop.avg_price)} / person
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {shop.sold.toLocaleString()} sold
            </Typography>
          </Box>
        </CardContent>
      </CardActionArea>
    </Card>
  );
};

export default ShopCard;
