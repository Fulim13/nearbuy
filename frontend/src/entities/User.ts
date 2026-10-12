export interface User {
  id: number;
  nickname: string;
  icon: string;
}

export interface FollowCounts {
  following: number;
  followers: number;
}
