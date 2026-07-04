import { NavLink, Route, Routes } from 'react-router-dom'
import Overview from './pages/Overview'
import PolicyWizard from './pages/PolicyWizard'
import Workloads from './pages/Workloads'
import AttestationEvidence from './pages/AttestationEvidence'
import KeyReleases from './pages/KeyReleases'
import AuditTimeline from './pages/AuditTimeline'
import Reports from './pages/Reports'

const NAV = [
  { to: '/', label: 'Overview' },
  { to: '/policies', label: 'Policy Wizard' },
  { to: '/workloads', label: 'Workloads' },
  { to: '/attestations', label: 'Attestation Evidence' },
  { to: '/key-releases', label: 'Key Releases' },
  { to: '/audit', label: 'Audit Timeline' },
  { to: '/reports', label: 'Reports' },
]

export default function App() {
  return (
    <>
      <nav className="nav">
        <span className="nav-brand">AI Sovereign Platform</span>
        {NAV.map(({ to, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) => `nav-link${isActive ? ' active' : ''}`}
          >
            {label}
          </NavLink>
        ))}
      </nav>
      <Routes>
        <Route path="/" element={<Overview />} />
        <Route path="/policies" element={<PolicyWizard />} />
        <Route path="/workloads" element={<Workloads />} />
        <Route path="/attestations" element={<AttestationEvidence />} />
        <Route path="/key-releases" element={<KeyReleases />} />
        <Route path="/audit" element={<AuditTimeline />} />
        <Route path="/reports" element={<Reports />} />
      </Routes>
    </>
  )
}
