import { useEffect, useState } from 'react'
import { api, type PlacementDecision } from '../api/client'

export default function Workloads() {
  const [items, setItems] = useState<PlacementDecision[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.workloads().then(setItems).catch((e: Error) => setError(e.message))
  }, [])

  if (error) return <div className="page"><div className="card" style={{color:'#f87171'}}>{error}</div></div>

  return (
    <div className="page">
      <h1 className="page-title">Workloads</h1>
      <div className="card">
        <table>
          <thead>
            <tr>
              <th>Pod</th>
              <th>Namespace</th>
              <th>Node</th>
              <th>Scheduler</th>
              <th>Decision</th>
              <th>Simulated</th>
              <th>Token Digest</th>
            </tr>
          </thead>
          <tbody>
            {items.length === 0 && (
              <tr><td colSpan={7} className="empty">No placement decisions found</td></tr>
            )}
            {items.map(d => (
              <tr key={d.metadata.name}>
                <td>{d.spec.targetRef?.name ?? d.metadata.name}</td>
                <td>{d.metadata.namespace}</td>
                <td>{d.status.nodeName || '—'}</td>
                <td>{d.spec.schedulerName}</td>
                <td>
                  <span className={`badge ${d.status.decision === 'allow' ? 'badge-green' : 'badge-red'}`}>
                    {d.status.decision || '—'}
                  </span>
                </td>
                <td>
                  {d.status.simulated
                    ? <span className="badge badge-yellow">SIMULATED</span>
                    : <span className="badge badge-green">REAL</span>}
                </td>
                <td style={{fontFamily:'monospace',fontSize:'0.75rem'}}>{d.status.placementTokenDigest?.slice(0,12) || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
