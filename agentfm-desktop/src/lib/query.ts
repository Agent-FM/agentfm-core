import { QueryClient, useQuery } from '@tanstack/react-query'
import { api } from './api'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

export const qk = {
  about: () => ['about'] as const,
  workers: (includeOffline: boolean) => ['workers', includeOffline] as const,
  peer: (peerId: string) => ['peer', peerId] as const,
  peerLog: (peerId: string, opts: { limit?: number; offset?: number }) =>
    ['peer-log', peerId, opts.limit ?? 50, opts.offset ?? 0] as const,
  commentBody: (peerId: string, cid: string) => ['comment-body', peerId, cid] as const,
}

export function useAbout() {
  return useQuery({ queryKey: qk.about(), queryFn: api.about, staleTime: 5000 })
}

export function useWorkers(includeOffline = false) {
  return useQuery({
    queryKey: qk.workers(includeOffline),
    queryFn: () => api.workers(includeOffline),
    staleTime: 2000,
    refetchInterval: 2000,
  })
}

export function usePeer(peerId: string | undefined) {
  return useQuery({
    queryKey: qk.peer(peerId ?? ''),
    queryFn: () => api.peer(peerId!),
    enabled: !!peerId,
    staleTime: 2000,
    refetchInterval: 5000,
  })
}

export function usePeerLog(
  peerId: string | undefined,
  opts: { limit?: number; offset?: number } = {},
) {
  return useQuery({
    queryKey: qk.peerLog(peerId ?? '', opts),
    queryFn: () => api.peerLog(peerId!, opts),
    enabled: !!peerId,
    staleTime: 5000,
  })
}

export function useCommentBody(peerId: string, cid: string | undefined) {
  return useQuery({
    queryKey: qk.commentBody(peerId, cid ?? ''),
    queryFn: () => api.commentBody(peerId, cid!),
    enabled: !!cid,
    staleTime: Infinity, // bodies are immutable
  })
}
