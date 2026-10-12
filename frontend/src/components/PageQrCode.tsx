import { Box, Typography } from '@mui/material';
import { QRCodeSVG } from 'qrcode.react';
import { useLocation } from 'react-router-dom';

interface Props {
  caption: string;
}

// Encodes the URL of the page it is rendered on. On localhost a phone cannot
// open it; run `npm run dev -- --host` and browse via the LAN address instead.
const PageQrCode = ({ caption }: Props) => {
  const location = useLocation();
  const url = window.location.origin + location.pathname;

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1, my: 2 }}>
      <Box sx={{ p: 1.5, bgcolor: 'white', border: '1px solid #EDE6DB', borderRadius: 2 }}>
        <QRCodeSVG value={url} size={168} marginSize={0} />
      </Box>
      <Typography variant="body2" color="text.secondary">
        {caption}
      </Typography>
      <Typography variant="caption" color="text.secondary" sx={{ wordBreak: 'break-all', textAlign: 'center' }}>
        {url}
      </Typography>
    </Box>
  );
};

export default PageQrCode;
