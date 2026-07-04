import { useState } from 'react'
import { api } from '../api/client'

interface PolicyForm {
  name: string
  namespace: string
  namespaceLabel: string
  appLabel: string
  requiredTEE: string[]
  requireConfidentialContainers: boolean
  allowedRuntimeClasses: string
  maxEvidenceAgeSeconds: number
  requireImageDigest: boolean
  requireModelDigest: boolean
  keyReleaseRequired: boolean
  keyReleaseTTL: number
  auditLevel: string
  enforcementMode: string
}

const defaultForm: PolicyForm = {
  name: '',
  namespace: 'default',
  namespaceLabel: 'high',
  appLabel: '',
  requiredTEE: ['TDX'],
  requireConfidentialContainers: true,
  allowedRuntimeClasses: 'kata-qemu-tdx,kata-qemu-snp',
  maxEvidenceAgeSeconds: 300,
  requireImageDigest: true,
  requireModelDigest: true,
  keyReleaseRequired: true,
  keyReleaseTTL: 300,
  auditLevel: 'full',
  enforcementMode: 'enforce',
}

function buildPolicyObject(form: PolicyForm) {
  return {
    apiVersion: 'aiops.imperium.io/v1alpha1',
    kind: 'ConfidentialInferencePolicy',
    metadata: { name: form.name || 'my-policy', namespace: form.namespace },
    spec: {
      target: {
        namespaceSelector: { matchLabels: { 'ai.sovereign.io/sensitivity': form.namespaceLabel } },
        workloadSelector: form.appLabel ? { matchLabels: { app: form.appLabel } } : undefined,
      },
      requiredTEE: form.requiredTEE,
      requireConfidentialContainers: form.requireConfidentialContainers,
      allowedRuntimeClasses: form.allowedRuntimeClasses.split(',').map(s => s.trim()).filter(Boolean),
      maxEvidenceAgeSeconds: form.maxEvidenceAgeSeconds,
      requireImageDigest: form.requireImageDigest,
      requireModelDigest: form.requireModelDigest,
      keyRelease: { required: form.keyReleaseRequired, ttlSeconds: form.keyReleaseTTL },
      audit: { required: true, level: form.auditLevel },
      enforcementMode: form.enforcementMode,
    },
  }
}

export default function PolicyWizard() {
  const [form, setForm] = useState<PolicyForm>(defaultForm)
  const [preview, setPreview] = useState<string | null>(null)
  const [status, setStatus] = useState<string | null>(null)

  function set<K extends keyof PolicyForm>(key: K, value: PolicyForm[K]) {
    setForm(f => ({ ...f, [key]: value }))
  }

  function handlePreview() {
    setPreview(JSON.stringify(buildPolicyObject(form), null, 2))
  }

  async function handleApply() {
    try {
      const result = await api.applyPolicy(buildPolicyObject(form))
      setStatus('Policy applied: ' + JSON.stringify(result))
    } catch (e) {
      setStatus('Error: ' + (e as Error).message)
    }
  }

  return (
    <div className="page">
      <h1 className="page-title">Policy Wizard</h1>
      <div className="grid grid-3">
        <div className="card">
          <h2>Identity</h2>
          <div className="form-group">
            <label>Policy Name</label>
            <input value={form.name} onChange={e => set('name', e.target.value)} placeholder="finance-confidential-llm" />
          </div>
          <div className="form-group">
            <label>Namespace</label>
            <input value={form.namespace} onChange={e => set('namespace', e.target.value)} />
          </div>
          <div className="form-group">
            <label>Namespace sensitivity label</label>
            <input value={form.namespaceLabel} onChange={e => set('namespaceLabel', e.target.value)} placeholder="high" />
          </div>
          <div className="form-group">
            <label>App label (workloadSelector)</label>
            <input value={form.appLabel} onChange={e => set('appLabel', e.target.value)} placeholder="risk-assistant" />
          </div>
        </div>

        <div className="card">
          <h2>TEE &amp; Runtime</h2>
          <div className="form-group">
            <label>Required TEE</label>
            {(['TDX', 'SEV-SNP'] as const).map(tee => (
              <label key={tee} style={{display:'flex',gap:'0.5rem',alignItems:'center',marginBottom:'0.25rem'}}>
                <input type="checkbox"
                  checked={form.requiredTEE.includes(tee)}
                  onChange={e => set('requiredTEE', e.target.checked
                    ? [...form.requiredTEE, tee]
                    : form.requiredTEE.filter(t => t !== tee))}
                />
                {tee}
              </label>
            ))}
          </div>
          <div className="form-group">
            <label>
              <input type="checkbox" checked={form.requireConfidentialContainers}
                onChange={e => set('requireConfidentialContainers', e.target.checked)} />
              {' '}Require Confidential Containers
            </label>
          </div>
          <div className="form-group">
            <label>Allowed Runtime Classes (comma-separated)</label>
            <input value={form.allowedRuntimeClasses} onChange={e => set('allowedRuntimeClasses', e.target.value)} />
          </div>
          <div className="form-group">
            <label>Max Evidence Age (seconds)</label>
            <input type="number" value={form.maxEvidenceAgeSeconds} onChange={e => set('maxEvidenceAgeSeconds', Number(e.target.value))} />
          </div>
        </div>

        <div className="card">
          <h2>Key Release &amp; Audit</h2>
          <div className="form-group">
            <label>
              <input type="checkbox" checked={form.requireImageDigest}
                onChange={e => set('requireImageDigest', e.target.checked)} />
              {' '}Require Image Digest
            </label>
          </div>
          <div className="form-group">
            <label>
              <input type="checkbox" checked={form.requireModelDigest}
                onChange={e => set('requireModelDigest', e.target.checked)} />
              {' '}Require Model Digest
            </label>
          </div>
          <div className="form-group">
            <label>
              <input type="checkbox" checked={form.keyReleaseRequired}
                onChange={e => set('keyReleaseRequired', e.target.checked)} />
              {' '}Key Release Required
            </label>
          </div>
          <div className="form-group">
            <label>Key Release TTL (seconds)</label>
            <input type="number" value={form.keyReleaseTTL} onChange={e => set('keyReleaseTTL', Number(e.target.value))} />
          </div>
          <div className="form-group">
            <label>Audit Level</label>
            <select value={form.auditLevel} onChange={e => set('auditLevel', e.target.value)}>
              <option value="summary">summary</option>
              <option value="full">full</option>
            </select>
          </div>
          <div className="form-group">
            <label>Enforcement Mode</label>
            <select value={form.enforcementMode} onChange={e => set('enforcementMode', e.target.value)}>
              <option value="warn">warn</option>
              <option value="audit">audit</option>
              <option value="enforce">enforce</option>
            </select>
          </div>
        </div>
      </div>

      <div style={{display:'flex',gap:'0.75rem',marginTop:'0.5rem'}}>
        <button className="btn btn-ghost" onClick={handlePreview}>Preview YAML</button>
        <button className="btn btn-primary" onClick={handleApply}>Apply Policy</button>
      </div>

      {status && <div className="card" style={{marginTop:'1rem',color:'#6ee7b7'}}>{status}</div>}

      {preview && (
        <div className="card" style={{marginTop:'1rem'}}>
          <h2>YAML Preview</h2>
          <pre>{preview}</pre>
        </div>
      )}
    </div>
  )
}
