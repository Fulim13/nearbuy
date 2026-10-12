import { Box } from '@mui/material';
import { useState } from 'react';
import ShopTypeIcon from './ShopTypeIcon';

const palettes = [
  ['#FFE8CC', '#E8590C'],
  ['#D3F9D8', '#2F9E44'],
  ['#D0EBFF', '#1971C2'],
  ['#F3D9FA', '#9C36B5'],
  ['#FFF3BF', '#E67700'],
  ['#FFDEEB', '#C2255C'],
];

interface Props {
  id: number;
  typeName?: string;
  // shop.images: comma separated; the first one is the cover.
  images?: string;
  height?: number;
}

// Seeded paths like /imgs/shops/1.jpg are served from frontend/public. A shop
// with no image, or whose file is missing, gets a coloured cover instead.
const ShopCover = ({ id, typeName, images, height = 120 }: Props) => {
  const src = images?.split(',')[0]?.trim();
  const [failedSrc, setFailedSrc] = useState<string | null>(null);
  const [bg, fg] = palettes[id % palettes.length];

  if (src && src !== failedSrc)
    return (
      <Box
        component="img"
        src={src}
        alt=""
        onError={() => setFailedSrc(src)}
        sx={{ display: 'block', width: '100%', height, objectFit: 'cover' }}
      />
    );

  return (
    <Box
      sx={{
        height,
        bgcolor: bg,
        color: fg,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <ShopTypeIcon name={typeName} sx={{ fontSize: height * 0.4 }} />
    </Box>
  );
};

export default ShopCover;
