import { Box, Button, Container, Typography } from '@mui/material';
import { isRouteErrorResponse, Link, useRouteError } from 'react-router-dom';
import NavBar from '../components/NavBar';

const ErrorPage = () => {
  const error = useRouteError();

  return (
    <>
      <NavBar />
      <Container maxWidth="sm" sx={{ py: 8, textAlign: 'center' }}>
        <Typography variant="h4">Oops</Typography>
        <Typography color="text.secondary" sx={{ my: 2 }}>
          {isRouteErrorResponse(error)
            ? 'This page does not exist.'
            : 'An unexpected error occurred.'}
        </Typography>
        <Box>
          <Button component={Link} to="/" variant="contained">Back to Explore</Button>
        </Box>
      </Container>
    </>
  );
};

export default ErrorPage;
