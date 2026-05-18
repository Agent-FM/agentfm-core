export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  rater_peer_id?: string; // for assistant messages: which worker responded
  timestamp: number;
}

export interface ChatSession {
  id: string;
  title: string; // auto-generated from first user message
  pinnedPeerId: string | null; // null = auto-route
  preferredModel: string; // model name for /v1/chat/completions routing; 'auto' = boss picks
  messages: ChatMessage[];
  createdAt: number;
  updatedAt: number;
}
