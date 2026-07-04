import { useEffect, useState } from 'react'
import { api } from '../api/client'

export default function KeyReleases() {
  const [items, setItems] = useState<unknown[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.keyReleases().then(setItems).catch((e: Error) => setError(e.message))
  }, [])

  if (error) return <div className="page"><div className="card" style={{color:'#f87171'}}>{error}</div></div>

  return (
    <div className="page">
      <h1 className="page-title">Key Releases</h1>
      <div className="card">
        {items.length === 0
          ? <p className="empty">No key release policies found</p>
          : <pre>{JSON.stringify(items, null, 2)}</pre>}
      </div>
    </div>
  )
}
