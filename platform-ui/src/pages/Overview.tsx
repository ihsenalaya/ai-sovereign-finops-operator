import { useEffect, useState } from 'react'
import { api, type Overview as OverviewData } from '../api/client'

export default function Overview() {
  const [data, setData] = useState<OverviewData | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.overview()
      .then(setData)
      .catch((e: Error) => setError(e.message))
  }, [])

  if (error) return <div className="page"><div className="card"><p style={{color:'#f87171'}}>Error: {error}</p></div></div>
  if (!data) return <div className="page"><div className="spinner" /></div>

  return (
    <div className="page">
      <h1 className="page-title">Overview</h1>

      {data.simulated_mode && (
        <div className="simulated-banner">
          ⚠ SIMULATED MODE — All TEE and GPU results are simulated in kind. Not for production use.
        </div>
      )}

      <div className="grid grid-4">
        <div className="card stat">
          <div className="stat-value">{data.total_policies}</div>
          <div className="stat-label">Policies</div>
        </div>
        <div className="card stat">
          <div className="stat-value">{data.valid_attestations}</div>
          <div className="stat-label">Valid Attestations</div>
        </div>
        <div className="card stat">
          <div className="stat-value" style={{color: data.expired_attestations > 0 ? '#f87171' : '#6ee7b7'}}>
            {data.expired_attestations}
          </div>
          <div className="stat-label">Expired Attestations</div>
        </div>
        <div className="card stat">
          <div className="stat-value" style={{color: data.revoked_attestations > 0 ? '#f87171' : '#6ee7b7'}}>
            {data.revoked_attestations}
          </div>
          <div className="stat-label">Revoked Attestations</div>
        </div>
        <div className="card stat">
          <div className="stat-value">{data.total_placement_decisions}</div>
          <div className="stat-label">Placement Decisions</div>
        </div>
        <div className="card stat">
          <div className="stat-value" style={{color: data.active_revocations > 0 ? '#f87171' : '#6ee7b7'}}>
            {data.active_revocations}
          </div>
          <div className="stat-label">Active Revocations</div>
        </div>
        <div className="card stat">
          <div className="stat-value">{data.total_evidence_records}</div>
          <div className="stat-label">Audit Records</div>
        </div>
        <div className="card stat">
          <div className="stat-value" style={{color: '#a78bfa'}}>
            {data.mode}
          </div>
          <div className="stat-label">Mode</div>
        </div>
      </div>

      <div className="card" style={{marginTop: '1rem', fontSize: '0.8rem', color: '#475569'}}>
        Last updated: {new Date(data.timestamp).toLocaleString()}
      </div>
    </div>
  )
}
