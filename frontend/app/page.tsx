import HealthBadge from './components/HealthBadge'

interface Listing {
  id: string
  title: string
  description?: string
}

async function fetchCatalog(): Promise<Listing[] | null> {
  const apiBase = process.env.NEXT_PUBLIC_API_BASE_URL
  if (!apiBase) return null
  try {
    const res = await fetch(`${apiBase}/listings`, {
      next: { revalidate: 60 },
    })
    if (!res.ok) return null
    const data = await res.json()
    return Array.isArray(data) ? data : null
  } catch {
    return null
  }
}

export default async function Home() {
  const listings = await fetchCatalog()

  return (
    <main>
      <header
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: '2rem',
          borderBottom: '1px solid #dee2e6',
          paddingBottom: '1rem',
        }}
      >
        <h1 style={{ margin: 0, fontSize: '1.5rem', fontWeight: 700 }}>ElaMachan</h1>
        <HealthBadge />
      </header>

      <section>
        <h2 style={{ fontSize: '1.125rem', fontWeight: 600, marginBottom: '1rem' }}>
          Catalog
        </h2>

        {listings && listings.length > 0 ? (
          <ul
            style={{
              listStyle: 'none',
              padding: 0,
              margin: 0,
              display: 'grid',
              gap: '1rem',
              gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))',
            }}
          >
            {listings.map(item => (
              <li
                key={item.id}
                style={{
                  background: '#fff',
                  border: '1px solid #dee2e6',
                  borderRadius: '0.5rem',
                  padding: '1rem',
                }}
              >
                <strong>{item.title}</strong>
                {item.description && (
                  <p style={{ margin: '0.5rem 0 0', color: '#6c757d', fontSize: '0.875rem' }}>
                    {item.description}
                  </p>
                )}
              </li>
            ))}
          </ul>
        ) : (
          <p style={{ color: '#6c757d', fontStyle: 'italic' }}>
            Catalog API not live yet — listings will appear here once the backend endpoint is available.
          </p>
        )}
      </section>
    </main>
  )
}
