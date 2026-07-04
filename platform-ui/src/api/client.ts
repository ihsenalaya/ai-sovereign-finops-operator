const BASE = '/api'

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`)
  if (!res.ok) throw new Error(`GET ${path}: ${res.status} ${res.statusText}`)
  return res.json()
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`POST ${path}: ${res.status} ${res.statusText}`)
  return res.json()
}

export interface Overview {
  timestamp: string
  mode: string
  total_policies: number
  total_attestations: number
  valid_attestations: number
  expired_attestations: number
  revoked_attestations: number
  total_placement_decisions: number
  active_revocations: number
  total_evidence_records: number
  simulated_mode: boolean
}

export interface Policy {
  metadata: { name: string; namespace: string }
  spec: Record<string, unknown>
  status: { policyHash?: string; simulated?: boolean; conditions?: unknown[] }
}

export interface Attestation {
  metadata: { name: string; namespace: string }
  spec: { subjectRef: { name: string }; tee: string; simulated?: boolean }
  status: { verified: boolean; revoked: boolean; lastVerifiedTime?: string }
}

export interface PlacementDecision {
  metadata: { name: string; namespace: string }
  spec: { targetRef: { name: string }; schedulerName: string }
  status: { decision: string; nodeName: string; simulated: boolean; placementTokenDigest?: string }
}

export interface AuditResponse {
  records: unknown[]
  chain_valid: boolean
  anchored: boolean
}

export interface TrustReport {
  timestamp: string
  trust_score: number
  total_placements: number
  active_revocations: number
  valid_evidences: number
  total_evidences: number
}

export const api = {
  overview: () => get<Overview>('/overview'),
  policies: () => get<Policy[]>('/policies'),
  previewPolicy: (spec: unknown) => post<unknown>('/policies/preview', spec),
  applyPolicy: (policy: unknown) => post<unknown>('/policies/apply', policy),
  attestations: () => get<Attestation[]>('/attestations'),
  keyReleases: () => get<unknown[]>('/key-releases'),
  revocations: () => get<unknown[]>('/revocations'),
  audit: () => get<AuditResponse>('/audit'),
  workloads: () => get<PlacementDecision[]>('/workloads'),
  trustReport: () => get<TrustReport>('/reports/trust'),
}
