import { Box, Container } from '@mui/material';
import { Outlet } from 'react-router-dom';
import NavBar from '../components/NavBar';

const Layout = () => {
  return (
    <>
      <NavBar />
      <Container maxWidth="lg" sx={{ py: 4 }}>
        <Box component="main">
          <Outlet />
        </Box>
      </Container>
    </>
  );
};

export default Layout;
