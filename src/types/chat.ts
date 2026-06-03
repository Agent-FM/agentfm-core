export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  rater_peer_id?: string; // for assistant messages: which worker responded
  task_id?: string; // boss-issued task id, set on assistant messages
  has_artifact?: boolean; // true once an artifact zip lands on disk for task_id
  timestamp: number;
}

export interface ChatSession {
  id: string;
  title: string;
  pinnedPeerId: string | null;
  messages: ChatMessage[];
  createdAt: number;
  updatedAt: number;
}
