import { useEffect, useState } from 'react'
import { api, type AuditResponse } from '../api/client'

export default function AuditTimeline() {
  const [data, setData] = useState<AuditResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.audit().then(setData).catch((e: Error) => setError(e.message))
  }, [])

  if (error) return <div className="page"><div className="card" style={{color:'#f87171'}}>{error}</div></div>
  if (!data) return <div className="page"><div className="spinner" /></div>

  return (
    <div className="page">
      <h1 className="page-title">Audit Timeline</h1>

      <div className="grid grid-3" style={{marginBottom:'1rem'}}>
        <div className="card stat">
          <div className="stat-value">{data.records.length}</div>
          <div className="stat-label">Audit Records</div>
        </div>
        <div className="card stat">
          <div className="stat-value" style={{color: data.chain_valid ? '#6ee7b7' : '#f87171'}}>
            {data.chain_valid ? 'VALID' : 'INVALID'}
          </div>
          <div className="stat-label">Chain Status</div>
        </div>
        <div className="card stat">
          <div className="stat-value" style={{color: data.anchored ? '#6ee7b7' : '#fcd34d'}}>
            {data.anchored ? 'ANCHORED' : 'PENDING'}
          </div>
          <div className="stat-label">Checkpoint</div>
        </div>
      </div>

      <div className="card">
        <h2>Records</h2>
        {data.records.length === 0
          ? <p className="empty">No audit records found</p>
          : <pre style={{maxHeight:'400px',overflowY:'auto'}}>{JSON.stringify(data.records, null, 2)}</pre>}
      </div>
    </div>
  )
}
