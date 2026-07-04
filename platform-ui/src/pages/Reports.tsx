import { useEffect, useState } from 'react'
import { api, type TrustReport } from '../api/client'

export default function Reports() {
  const [report, setReport] = useState<TrustReport | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.trustReport().then(setReport).catch((e: Error) => setError(e.message))
  }, [])

  if (error) return <div className="page"><div className="card" style={{color:'#f87171'}}>{error}</div></div>
  if (!report) return <div className="page"><div className="spinner" /></div>

  const score = Math.round(report.trust_score * 100)

  return (
    <div className="page">
      <h1 className="page-title">Trust Report</h1>

      <div className="grid grid-4">
        <div className="card stat">
          <div className="stat-value" style={{color: score >= 80 ? '#6ee7b7' : score >= 50 ? '#fcd34d' : '#f87171'}}>
            {score}%
          </div>
          <div className="stat-label">Trust Score</div>
        </div>
        <div className="card stat">
          <div className="stat-value">{report.total_placements}</div>
          <div className="stat-label">Total Placements</div>
        </div>
        <div className="card stat">
          <div className="stat-value" style={{color: report.active_revocations > 0 ? '#f87171' : '#6ee7b7'}}>
            {report.active_revocations}
          </div>
          <div className="stat-label">Active Revocations</div>
        </div>
        <div className="card stat">
          <div className="stat-value">{report.valid_evidences}/{report.total_evidences}</div>
          <div className="stat-label">Valid Evidences</div>
        </div>
      </div>

      <div className="card" style={{marginTop:'1rem'}}>
        <h2>Raw Report</h2>
        <pre>{JSON.stringify(report, null, 2)}</pre>
      </div>
    </div>
  )
}
