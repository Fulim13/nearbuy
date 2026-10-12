export interface User {
  id: number;
  nickname: string;
  icon: string;
  // Only the session user (login, /user/me) carries a role; public profiles don't.
  role?: 'user' | 'admin';
}

export interface FollowCounts {
  following: number;
  followers: number;
}
