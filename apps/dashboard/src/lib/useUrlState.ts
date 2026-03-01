import type { DashboardParams } from './params'
import { useCallback, useEffect, useState } from 'react'
import { buildSearch, parseParams } from './params'

export function useUrlState() {
  const [urlParams, setUrlParams] = useState<DashboardParams>(() =>
    parseParams(window.location.search),
  )

  useEffect(() => {
    function onPop() {
      setUrlParams(parseParams(window.location.search))
    }
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  const setParams = useCallback((patch: Partial<DashboardParams>) => {
    setUrlParams((prev) => {
      const next: DashboardParams = { ...prev, ...patch }
      const search = buildSearch(next)
      window.history.replaceState(null, '', search ? `?${search}` : window.location.pathname)
      return next
    })
  }, [])

  return [urlParams, setParams] as const
}
