// ---- Account ----
export interface User {
  id: number;
  email: string;
  nickname: string;
  avatar_url: string;
  bio: string;
  role: string;
  status: string;
  created_at: string;
}

export interface Session {
  token: string;
  user: User;
}

// ---- Notes ----
export interface ImageDTO {
  id: number;
  url: string;
  width?: number;
  height?: number;
}

export interface Note {
  id: number;
  title: string;
  body: string;
  status: string;
  visibility: string;
  author: { id: number; nickname: string; avatar_url: string };
  images: ImageDTO[];
  tags: string[];
  likes_count: number;
  favorites_count: number;
  comments_count: number;
  published_at?: string;
  created_at: string;
  updated_at: string;
}

export interface NoteList {
  items: Note[];
  total: number;
}

// ---- Feed ----
export interface FeedItem {
  id: number;
  title: string;
  body: string;
  author_id: number;
  author_nickname: string;
  author_avatar: string;
  likes_count: number;
  favorites_count: number;
  comments_count: number;
  tags: string[];
  image_urls: string[];
  published_at?: string;
  created_at: string;
  viewer_liked?: boolean | null;
  viewer_favorited?: boolean | null;
  viewer_following?: boolean | null;
}

export interface FeedResult {
  items: FeedItem[];
  next_cursor: string;
  has_more: boolean;
}

// ---- Comments ----
export interface Comment {
  id: number;
  note_id: number;
  author_id: number;
  author_nickname: string;
  body: string;
  created_at: string;
}

export interface CommentList {
  items: Comment[];
  total: number;
}

// ---- Review ----
export interface ReviewTask {
  id: number;
  note_id: number;
  author_id: number;
  status: string;
  source: string;
  agent_decision?: string;
  agent_risk_level?: string;
  agent_reason?: string;
  agent_trace_id?: string;
  admin_decision?: string;
  admin_reason?: string;
  decided_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ReviewTaskList {
  items: ReviewTask[];
  total: number;
}

// ---- API envelope ----
export interface ApiEnvelope<T> {
  data: T;
  error: { code: string; message: string } | null;
  request_id: string;
}
