'use client'

import { useEffect, useState } from 'react'

type Status = 'loading' | 'healthy' | 'unreachable'

const DOT: Record<Status, string> = {
  loading: '#aaa',
  healthy: '#22c55e',
  unreachable: '#ef4444',
}

const LABEL: Record<Status, string> = {
  loading: 'Checking…',
  healthy: 'Backend healthy',
  unreachable: 'Backend unreachable',
}

export default function HealthBadge() {
  const [status, setStatus] = useState<Status>('loading')
  const apiBase = process.env.NEXT_PUBLIC_API_BASE_URL

  useEffect(() => {
    if (!apiBase) {
      setStatus('unreachable')
      return
    }
    const controller = new AbortController()
    fetch(`${apiBase}/healthz`, { signal: controller.signal, cache: 'no-store' })
      .then(r => setStatus(r.ok ? 'healthy' : 'unreachable'))
      .catch(() => setStatus('unreachable'))
    return () => controller.abort()
  }, [apiBase])

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '0.4rem',
        padding: '0.25rem 0.75rem',
        border: '1px solid #dee2e6',
        borderRadius: '999px',
        fontSize: '0.875rem',
        background: '#fff',
      }}
    >
      <span
        style={{
          width: 10,
          height: 10,
          borderRadius: '50%',
          background: DOT[status],
          flexShrink: 0,
        }}
      />
      {LABEL[status]}
    </span>
  )
}
