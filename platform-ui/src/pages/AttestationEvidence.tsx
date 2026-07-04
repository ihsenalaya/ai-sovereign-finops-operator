import { useEffect, useState } from 'react'
import { api, type Attestation } from '../api/client'

export default function AttestationEvidence() {
  const [items, setItems] = useState<Attestation[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.attestations().then(setItems).catch((e: Error) => setError(e.message))
  }, [])

  if (error) return <div className="page"><div className="card" style={{color:'#f87171'}}>{error}</div></div>

  return (
    <div className="page">
      <h1 className="page-title">Attestation Evidence</h1>
      <div className="card">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Subject (Node)</th>
              <th>TEE</th>
              <th>Verified</th>
              <th>Revoked</th>
              <th>Mode</th>
              <th>Last Verified</th>
            </tr>
          </thead>
          <tbody>
            {items.length === 0 && (
              <tr><td colSpan={7} className="empty">No attestation evidence found</td></tr>
            )}
            {items.map(ev => (
              <tr key={ev.metadata.name}>
                <td>{ev.metadata.name}</td>
                <td>{ev.spec.subjectRef?.name || '—'}</td>
                <td><span className="badge badge-blue">{ev.spec.tee}</span></td>
                <td>
                  <span className={`badge ${ev.status.verified ? 'badge-green' : 'badge-red'}`}>
                    {ev.status.verified ? 'YES' : 'NO'}
                  </span>
                </td>
                <td>
                  <span className={`badge ${ev.status.revoked ? 'badge-red' : 'badge-green'}`}>
                    {ev.status.revoked ? 'REVOKED' : 'OK'}
                  </span>
                </td>
                <td>
                  {ev.spec.simulated
                    ? <span className="badge badge-yellow">SIMULATED</span>
                    : <span className="badge badge-green">REAL</span>}
                </td>
                <td style={{fontSize:'0.8rem'}}>
                  {ev.status.lastVerifiedTime
                    ? new Date(ev.status.lastVerifiedTime).toLocaleString()
                    : '—'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
