import {
  Alert,
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  Divider,
  List,
  ListItemAvatar,
  ListItemButton,
  ListItemText,
  Skeleton,
  Stack,
  Typography,
} from '@mui/material';
import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import type { FollowCounts, User } from '../entities/User';
import useData from '../hooks/useData';
import apiClient from '../services/api-client';
import { getErrorMessage } from '../services/errors';
import useAuthStore, { useIsAdmin } from '../stores/authStore';

const UserPage = () => {
  const { id } = useParams();
  const me = useAuthStore((s) => s.user);
  const isAdmin = useIsAdmin();
  const isMe = me?.id === Number(id);
  // Admin accounts run the catalogue; the gateway refuses their follow calls.
  const canFollow = !!me && !isMe && !isAdmin;

  const { data: user, error, isLoading } = useData<User>(`/user/${id}`);
  const { data: counts, setData: setCounts } = useData<FollowCounts>(`/users/${id}/follow-counts`);
  const { data: following, setData: setFollowing } = useData<{ following: boolean }>(
    canFollow ? `/users/${id}/follow` : null
  );
  const { data: common, refetch: refetchCommon } = useData<User[]>(
    canFollow ? `/users/${id}/common-following` : null
  );
  const [isBusy, setBusy] = useState(false);
  const [actionError, setActionError] = useState('');

  const toggleFollow = () => {
    if (!following || !counts) return;
    const willFollow = !following.following;
    setBusy(true);
    setActionError('');

    const request = willFollow
      ? apiClient.put(`/users/${id}/follow`)
      : apiClient.delete(`/users/${id}/follow`);

    request
      .then(() => {
        setFollowing({ following: willFollow });
        setCounts({ ...counts, followers: counts.followers + (willFollow ? 1 : -1) });
        refetchCommon();
      })
      .catch((err) => setActionError(getErrorMessage(err)))
      .finally(() => setBusy(false));
  };

  if (isLoading) return <Skeleton variant="rounded" height={240} />;
  if (error || !user) return <Alert severity="error">{error || 'User not found'}</Alert>;

  return (
    <Card sx={{ maxWidth: 560, mx: 'auto' }}>
      <CardContent sx={{ p: 4 }}>
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
          <Avatar src={user.icon || undefined} alt={user.nickname} sx={{ width: 72, height: 72, bgcolor: 'primary.main', fontSize: 30 }}>
            {user.nickname.charAt(0).toUpperCase()}
          </Avatar>
          <Box sx={{ flexGrow: 1 }}>
            <Typography variant="h5">{user.nickname}</Typography>
            <Typography color="text.secondary">User #{user.id}</Typography>
          </Box>
          {canFollow && following && (
            <Button
              variant={following.following ? 'outlined' : 'contained'}
              onClick={toggleFollow}
              loading={isBusy}
            >
              {following.following ? 'Unfollow' : 'Follow'}
            </Button>
          )}
        </Stack>

        {isMe && <Alert severity="info" sx={{ mt: 2 }}>This is you.</Alert>}
        {isAdmin && !isMe && (
          <Alert severity="info" sx={{ mt: 2 }}>Admin accounts can't follow people.</Alert>
        )}
        {!me && (
          <Alert severity="info" sx={{ mt: 2 }}>
            <Link to="/login" state={{ from: `/users/${id}` }}>Log in</Link> to follow {user.nickname}.
          </Alert>
        )}
        {actionError && <Alert severity="error" sx={{ mt: 2 }}>{actionError}</Alert>}

        {counts && (
          <Stack direction="row" spacing={4} sx={{ mt: 3 }}>
            <Box>
              <Typography variant="h6">{counts.followers}</Typography>
              <Typography variant="caption" color="text.secondary">Followers</Typography>
            </Box>
            <Box>
              <Typography variant="h6">{counts.following}</Typography>
              <Typography variant="caption" color="text.secondary">Following</Typography>
            </Box>
          </Stack>
        )}

        {canFollow && common && (
          <>
            <Divider sx={{ my: 3 }} />
            <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>
              You both follow
            </Typography>
            {common.length === 0 ? (
              <Typography color="text.secondary" variant="body2">Nobody in common yet.</Typography>
            ) : (
              <List dense>
                {common.map((u) => (
                  <ListItemButton key={u.id} component={Link} to={`/users/${u.id}`}>
                    <ListItemAvatar>
                      <Avatar src={u.icon || undefined} alt={u.nickname}>
                        {u.nickname.charAt(0).toUpperCase()}
                      </Avatar>
                    </ListItemAvatar>
                    <ListItemText primary={u.nickname} secondary={`User #${u.id}`} />
                  </ListItemButton>
                ))}
              </List>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
};

export default UserPage;
