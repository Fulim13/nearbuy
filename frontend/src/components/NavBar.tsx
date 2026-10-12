import LocalMallIcon from '@mui/icons-material/LocalMall';
import {
  AppBar,
  Avatar,
  Box,
  Button,
  Chip,
  Container,
  Menu,
  MenuItem,
  Toolbar,
  Typography,
} from '@mui/material';
import { useState } from 'react';
import { Link, NavLink, useNavigate } from 'react-router-dom';
import apiClient from '../services/api-client';
import useAuthStore, { useIsAdmin } from '../stores/authStore';
import SearchInput from './SearchInput';

const navLinkSx = {
  color: 'inherit',
  opacity: 0.85,
  '&.active': { opacity: 1, bgcolor: 'rgba(255,255,255,0.16)' },
};

const NavBar = () => {
  const { user, logout } = useAuthStore();
  const isAdmin = useIsAdmin();
  const navigate = useNavigate();
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);

  const handleLogout = () => {
    setAnchor(null);
    // Clear the local session only once the request has been sent: the request
    // interceptor reads the token asynchronously, so logging out first would
    // send the logout without it and leave the session alive in Redis.
    apiClient
      .post('/user/logout')
      .catch(() => {})
      .finally(() => {
        // Leave the page first, or PrivateRoutes sees no user and redirects to
        // /login. flushSync: the router otherwise commits in a transition,
        // after the (synchronous) store update has already re-rendered.
        navigate('/', { flushSync: true });
        logout();
      });
  };

  return (
    // Admins get a navy header so it is obvious which kind of session this is.
    <AppBar position="sticky" sx={{ bgcolor: isAdmin ? 'secondary.main' : 'primary.main' }}>
      <Container maxWidth="lg">
        <Toolbar disableGutters sx={{ gap: 2, flexWrap: 'wrap', py: 1 }}>
          <Box
            component={Link}
            to="/"
            sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'inherit', textDecoration: 'none' }}
          >
            <LocalMallIcon />
            <Typography variant="h6" sx={{ fontWeight: 800 }}>
              NearBuy
            </Typography>
            {isAdmin && (
              <Chip
                label="ADMIN"
                size="small"
                sx={{ bgcolor: 'primary.main', color: 'white', fontWeight: 800, letterSpacing: '0.06em' }}
              />
            )}
          </Box>

          <SearchInput />

          <Box sx={{ display: 'flex', gap: 0.5, ml: 'auto', alignItems: 'center' }}>
            <Button component={NavLink} to="/" end sx={navLinkSx}>
              Explore
            </Button>
            <Button component={NavLink} to="/users" sx={navLinkSx}>
              People
            </Button>
            {user && !isAdmin && (
              <Button component={NavLink} to="/orders" sx={navLinkSx}>
                My Orders
              </Button>
            )}

            {user ? (
              <>
                <Button
                  color="inherit"
                  onClick={(e) => setAnchor(e.currentTarget)}
                  startIcon={
                    <Avatar
                      src={user.icon || undefined}
                      alt={user.nickname}
                      sx={{ width: 26, height: 26, bgcolor: isAdmin ? 'primary.main' : 'secondary.main', fontSize: 13 }}
                    >
                      {user.nickname.charAt(0).toUpperCase()}
                    </Avatar>
                  }
                >
                  {user.nickname}
                </Button>
                <Menu anchorEl={anchor} open={!!anchor} onClose={() => setAnchor(null)}>
                  <MenuItem
                    onClick={() => {
                      setAnchor(null);
                      navigate(`/users/${user.id}`);
                    }}
                  >
                    My profile (#{user.id})
                  </MenuItem>
                  {isAdmin && (
                    <MenuItem
                      onClick={() => {
                        setAnchor(null);
                        navigate('/shops/new');
                      }}
                    >
                      Open a shop
                    </MenuItem>
                  )}
                  <MenuItem onClick={handleLogout}>Log out</MenuItem>
                </Menu>
              </>
            ) : (
              <Button component={Link} to="/login" variant="contained" color="secondary">
                Log in
              </Button>
            )}
          </Box>
        </Toolbar>
      </Container>
    </AppBar>
  );
};

export default NavBar;
