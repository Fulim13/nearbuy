import { Box } from '@mui/material';
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
  height?: number;
}

// Shop images in the seed data point at files that do not exist, so every
// shop gets a deterministic coloured cover instead.
const ShopCover = ({ id, typeName, height = 120 }: Props) => {
  const [bg, fg] = palettes[id % palettes.length];

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
