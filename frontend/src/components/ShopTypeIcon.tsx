import BrushIcon from '@mui/icons-material/Brush';
import CelebrationIcon from '@mui/icons-material/Celebration';
import ChildCareIcon from '@mui/icons-material/ChildCare';
import ContentCutIcon from '@mui/icons-material/ContentCut';
import FitnessCenterIcon from '@mui/icons-material/FitnessCenter';
import LocalBarIcon from '@mui/icons-material/LocalBar';
import MicIcon from '@mui/icons-material/Mic';
import RestaurantIcon from '@mui/icons-material/Restaurant';
import SelfImprovementIcon from '@mui/icons-material/SelfImprovement';
import SpaIcon from '@mui/icons-material/Spa';
import StorefrontIcon from '@mui/icons-material/Storefront';
import type { SvgIconProps } from '@mui/material';
import type { ComponentType } from 'react';

// The seeded icon paths (/types/ms.png, ...) are not served anywhere, so map
// the type name onto a Material icon instead.
const iconMap: Record<string, ComponentType<SvgIconProps>> = {
  'Food': RestaurantIcon,
  'KTV': MicIcon,
  'Hair Salon': ContentCutIcon,
  'Gym': FitnessCenterIcon,
  'Massage': SelfImprovementIcon,
  'Beauty SPA': SpaIcon,
  'Kids Play': ChildCareIcon,
  'Bar': LocalBarIcon,
  'Party House': CelebrationIcon,
  'Nails': BrushIcon,
};

interface Props extends SvgIconProps {
  name?: string;
}

const ShopTypeIcon = ({ name, ...props }: Props) => {
  const Icon = (name && iconMap[name]) || StorefrontIcon;
  return <Icon {...props} />;
};

export default ShopTypeIcon;
