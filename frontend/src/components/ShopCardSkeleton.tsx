import { Card, CardContent, Skeleton } from '@mui/material';

const ShopCardSkeleton = () => {
  return (
    <Card>
      <Skeleton variant="rectangular" height={120} />
      <CardContent>
        <Skeleton width="70%" />
        <Skeleton width="40%" />
        <Skeleton width="55%" />
      </CardContent>
    </Card>
  );
};

export default ShopCardSkeleton;
