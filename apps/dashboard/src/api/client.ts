// Custom fetch instance for orval-generated API clients.
// Call setApiToken(token) once after the user authenticates (see TokenGate.tsx).
let _token = ''

export function setApiToken(token: string) {
  _token = token
}

// Signature matches what orval's fetch client generator expects:
// customInstance<T>(url, options?: RequestInit) — URL includes query params.
export const customInstance = async <T>(
  url: string,
  options?: RequestInit,
): Promise<T> => {
  const res = await fetch(url, {
    ...options,
    headers: {
      ...options?.headers,
      Authorization: `Bearer ${_token}`,
    },
  })

  if (!res.ok)
    throw new Error(`${res.status}`)

  return res.json() as Promise<T>
}

export default customInstance
