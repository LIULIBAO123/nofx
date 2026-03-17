import { useEffect, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import useSWR, { mutate as swrMutate } from 'swr'
import { api } from './lib/api'
import { TraderDashboardPage } from './pages/TraderDashboardPage'
import { LongShortRadarPage } from './pages/LongShortRadarPage'

import { AITradersPage } from './components/AITradersPage'
import { LoginPage } from './components/LoginPage'
import { RegisterPage } from './components/RegisterPage'
import { ResetPasswordPage } from './components/ResetPasswordPage'
import { CompetitionPage } from './components/CompetitionPage'
import { LandingPage } from './pages/LandingPage'
import { StrategyStudioPage } from './pages/StrategyStudioPage'
import { LoginRequiredOverlay } from './components/LoginRequiredOverlay'
import HeaderBar from './components/HeaderBar'
import { LanguageProvider, useLanguage } from './contexts/LanguageContext'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { ConfirmDialogProvider } from './components/ConfirmDialog'
import { t } from './i18n/translations'
import { useSystemConfig } from './hooks/useSystemConfig'

import { OFFICIAL_LINKS } from './constants/branding'
import { BacktestPage } from './components/BacktestPage'
import { DataStatisticsPage } from './pages/DataStatisticsPage'
import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  Exchange,
  LatestAnalysisResponse,
  DirectionPoolResponse,
} from './types'

type Page =
  | 'competition'
  | 'traders'
  | 'trader'
  | 'simulation'
  | 'backtest'
  | 'strategy'
  | 'radar'
  | 'datastats'
  | 'login'
  | 'register'



function App() {
  const { language, setLanguage } = useLanguage()
  const { user, token, logout, isLoading } = useAuth()
  const { loading: configLoading } = useSystemConfig()
  const [route, setRoute] = useState(window.location.pathname)

  // Debug log
  useEffect(() => {
    console.log('[App] Mounted. Route:', window.location.pathname);
  }, []);

  // 从URL路径读取初始页面状态（支持刷新保持页面）
  const getInitialPage = (): Page => {
    const path = window.location.pathname
    const hash = window.location.hash.slice(1) // 去掉 #

    if (path === '/traders' || hash === 'traders') return 'traders'
    if (path === '/simulation' || hash === 'simulation') return 'simulation'
    if (path === '/radar' || hash === 'radar') return 'radar'
    if (path === '/backtest' || hash === 'backtest') return 'backtest'
    if (path === '/strategy' || hash === 'strategy') return 'strategy'
    if (path === '/datastats' || hash === 'datastats') return 'datastats'
    if (path === '/dashboard' || hash === 'trader' || hash === 'details')
      return 'trader'
    return 'competition' // 默认为竞赛页面
  }

  // Login required overlay state
  const [loginOverlayOpen, setLoginOverlayOpen] = useState(false)
  const [loginOverlayFeature, setLoginOverlayFeature] = useState('')

  const handleLoginRequired = (featureName: string) => {
    setLoginOverlayFeature(featureName)
    setLoginOverlayOpen(true)
  }

  // Unified page navigation handler
  const navigateToPage = (page: Page) => {
    const pathMap: Record<Page, string> = {
      'competition': '/competition',
      'traders': '/traders',
      'trader': '/dashboard',
      'simulation': '/simulation',
      'radar': '/radar',
      'backtest': '/backtest',
      'strategy': '/strategy',
      'datastats': '/datastats',
      'login': '/login',
      'register': '/register',
    }
    const path = pathMap[page]
    if (path) {
      window.history.pushState({}, '', path)
      setRoute(path)
      setCurrentPage(page)
    }
  }

  const [currentPage, setCurrentPage] = useState<Page>(getInitialPage())

  // 直接访问 /radar 等路径时确保 currentPage 与 pathname 一致（含刷新、新开标签）
  useEffect(() => {
    const path = window.location.pathname
    if (path === '/radar' && currentPage !== 'radar') setCurrentPage('radar')
    if (path === '/strategy' && currentPage !== 'strategy') setCurrentPage('strategy')
    if (path === '/datastats' && currentPage !== 'datastats') setCurrentPage('datastats')
    if (path === '/backtest' && currentPage !== 'backtest') setCurrentPage('backtest')
    if (path === '/traders' && currentPage !== 'traders') setCurrentPage('traders')
    if (path === '/dashboard' && currentPage !== 'trader') setCurrentPage('trader')
    if (path === '/simulation' && currentPage !== 'simulation') setCurrentPage('simulation')
    if ((path === '/' || path === '') && currentPage !== 'competition') setCurrentPage('competition')
  }, [])

  // 从 URL 参数读取初始 trader 标识（格式: name-id前4位）
  const [selectedTraderSlug, setSelectedTraderSlug] = useState<string | undefined>(() => {
    const params = new URLSearchParams(window.location.search)
    return params.get('trader') || undefined
  })
  const [selectedTraderId, setSelectedTraderId] = useState<string | undefined>()

  // 生成 trader URL slug（name + ID 前 4 位）
  const getTraderSlug = (trader: TraderInfo) => {
    const idPrefix = trader.trader_id.slice(0, 4)
    return `${trader.trader_name}-${idPrefix}`
  }

  // 从 slug 解析并匹配 trader
  const findTraderBySlug = (slug: string, traderList: TraderInfo[]) => {
    // slug 格式: name-xxxx (xxxx 是 ID 前 4 位)
    const lastDashIndex = slug.lastIndexOf('-')
    if (lastDashIndex === -1) {
      // 没有 dash，直接按 name 匹配
      return traderList.find(t => t.trader_name === slug)
    }
    const name = slug.slice(0, lastDashIndex)
    const idPrefix = slug.slice(lastDashIndex + 1)
    return traderList.find(t =>
      t.trader_name === name && t.trader_id.startsWith(idPrefix)
    )
  }
  const [lastUpdate, setLastUpdate] = useState<string>('--:--:--')
  const [decisionsLimit, setDecisionsLimit] = useState<number>(5)

  // 监听URL变化，同步页面状态
  useEffect(() => {
    const handleRouteChange = () => {
      const path = window.location.pathname
      const hash = window.location.hash.slice(1)
      const params = new URLSearchParams(window.location.search)
      const traderParam = params.get('trader')

      if (path === '/traders' || hash === 'traders') {
        setCurrentPage('traders')
      } else if (path === '/radar' || hash === 'radar') {
        setCurrentPage('radar')
      } else if (path === '/simulation' || hash === 'simulation') {
        setCurrentPage('simulation')
        if (traderParam) setSelectedTraderSlug(traderParam)
      } else if (path === '/backtest' || hash === 'backtest') {
        setCurrentPage('backtest')
      } else if (path === '/strategy' || hash === 'strategy') {
        setCurrentPage('strategy')
      } else if (path === '/datastats' || hash === 'datastats') {
        setCurrentPage('datastats')
      } else if (
        path === '/dashboard' ||
        hash === 'trader' ||
        hash === 'details'
      ) {
        setCurrentPage('trader')
        // 如果 URL 中有 trader 参数（slug 格式），更新选中的 trader
        if (traderParam) {
          setSelectedTraderSlug(traderParam)
        }
      } else if (
        path === '/competition' ||
        hash === 'competition' ||
        hash === ''
      ) {
        setCurrentPage('competition')
      }
      setRoute(path)
    }

    window.addEventListener('hashchange', handleRouteChange)
    window.addEventListener('popstate', handleRouteChange)
    return () => {
      window.removeEventListener('hashchange', handleRouteChange)
      window.removeEventListener('popstate', handleRouteChange)
    }
  }, [])

  // 切换页面时更新URL hash (当前通过按钮直接调用setCurrentPage，这个函数暂时保留用于未来扩展)
  // const navigateToPage = (page: Page) => {
  //   setCurrentPage(page);
  //   window.location.hash = page === 'competition' ? '' : 'trader';
  // };

  // 获取实盘 trader 列表（仅在实盘相关页需要）
  const { data: traders, error: tradersError, isLoading: tradersLoading } = useSWR<TraderInfo[]>(
    user && token && (currentPage === 'traders' || currentPage === 'trader' || currentPage === 'radar' || currentPage === 'datastats') ? 'traders' : null,
    () => api.getTraders(),
    {
      refreshInterval: 10000,
      shouldRetryOnError: false, // 避免在后端未运行时无限重试
    }
  )

  // 获取交易所列表（实盘与实盘模拟分离：实盘用 / 模拟用 分别请求）
  const { data: exchanges } = useSWR<Exchange[]>(
    user && token && (currentPage === 'traders' || currentPage === 'trader') ? 'exchanges' : null,
    () => api.getExchangeConfigs({ simulation: false }),
    { refreshInterval: 60000, shouldRetryOnError: false }
  )
  const { data: simulationExchanges } = useSWR<Exchange[]>(
    user && token && currentPage === 'simulation' ? 'exchanges-simulation' : null,
    () => api.getExchangeConfigs({ simulation: true }),
    { refreshInterval: 60000, shouldRetryOnError: false }
  )

  // 获取实盘模拟 trader 列表（实盘模拟页 + 多空雷达页均需要；radar 用独立 key 确保进入多空雷达时必定请求）
  const simulationTradersKey =
    user && token && currentPage === 'simulation' ? 'simulation-traders' :
    user && token && currentPage === 'radar' ? 'radar-simulation-traders' :
    user && token && currentPage === 'datastats' ? 'datastats-simulation-traders' : null
  const { data: simulationTraders, isLoading: simulationTradersLoading } = useSWR<TraderInfo[]>(
    simulationTradersKey,
    () => api.getTraders({ simulation: true }),
    { refreshInterval: 10000, shouldRetryOnError: false }
  )

  // 当获取到 traders 后，根据 URL 中的 trader slug 或默认选中第一个（实盘看板 / 多空雷达）
  const tradersForSelection = currentPage === 'simulation' ? simulationTraders : traders
  const radarTradersCombined = currentPage === 'radar' ? [...(traders ?? []), ...(simulationTraders ?? [])] : []
  useEffect(() => {
    if (currentPage === 'radar') {
      if (radarTradersCombined.length === 0) return
      if (selectedTraderSlug) {
        const trader = findTraderBySlug(selectedTraderSlug, radarTradersCombined)
        if (trader) setSelectedTraderId(trader.trader_id)
        else setSelectedTraderId(radarTradersCombined[0].trader_id)
      } else if (!selectedTraderId) {
        setSelectedTraderId(radarTradersCombined[0].trader_id)
      }
      return
    }
    if (!tradersForSelection || tradersForSelection.length === 0) return
    if (selectedTraderSlug) {
      const trader = findTraderBySlug(selectedTraderSlug, tradersForSelection)
      if (trader) setSelectedTraderId(trader.trader_id)
      else if (currentPage === 'trader') setSelectedTraderId(tradersForSelection[0].trader_id)
    } else if (currentPage === 'trader' && !selectedTraderId) {
      setSelectedTraderId(tradersForSelection[0].trader_id)
    }
  }, [tradersForSelection, radarTradersCombined, selectedTraderSlug, currentPage, selectedTraderId])

  // 在实盘或实盘模拟看板时，获取该 trader 的数据（8 秒轮询以便更接近实时）
  // 约定：实盘模拟看板的更新（轮询间隔、刷新、lastUpdate 等）与实盘看板保持一致，不单独分支
  const isDashboard = currentPage === 'trader' || currentPage === 'simulation'
  const isRadar = currentPage === 'radar'
  const { data: status } = useSWR<SystemStatus>(
    (isDashboard || isRadar) && selectedTraderId ? `status-${selectedTraderId}` : null,
    () => api.getStatus(selectedTraderId!),
    {
      refreshInterval: 8000, // 8 秒刷新，看板信息更及时
      revalidateOnFocus: true, // 切回标签页时立即刷新
      dedupingInterval: 5000,
    }
  )

  const { data: account } = useSWR<AccountInfo>(
    isDashboard && selectedTraderId ? `account-${selectedTraderId}` : null,
    () => api.getAccount(selectedTraderId!),
    {
      refreshInterval: 8000,
      revalidateOnFocus: true,
      dedupingInterval: 5000,
    }
  )

  const { data: positions } = useSWR<Position[]>(
    isDashboard && selectedTraderId ? `positions-${selectedTraderId}` : null,
    () => api.getPositions(selectedTraderId!),
    {
      refreshInterval: 8000,
      revalidateOnFocus: true,
      dedupingInterval: 5000,
    }
  )

  const { data: decisions } = useSWR<DecisionRecord[]>(
    isDashboard && selectedTraderId
      ? `decisions/latest-${selectedTraderId}-${decisionsLimit}`
      : null,
    () => api.getLatestDecisions(selectedTraderId!, decisionsLimit),
    {
      refreshInterval: 20000,
      revalidateOnFocus: true,
      dedupingInterval: 10000,
    }
  )

  const { data: stats } = useSWR<Statistics>(
    isDashboard && selectedTraderId ? `statistics-${selectedTraderId}` : null,
    () => api.getStatistics(selectedTraderId!),
    {
      refreshInterval: 20000,
      revalidateOnFocus: true,
      dedupingInterval: 10000,
    }
  )

  const { data: latestAnalysis } = useSWR<LatestAnalysisResponse>(
    isDashboard && selectedTraderId ? `latest-analysis-${selectedTraderId}` : null,
    () => api.getLatestAnalysis(selectedTraderId!),
    { refreshInterval: 15000, revalidateOnFocus: true, dedupingInterval: 8000 }
  )

  const { data: directionPool } = useSWR<DirectionPoolResponse>(
    isDashboard && selectedTraderId ? `direction-pool-${selectedTraderId}` : null,
    () => api.getDirectionPool(selectedTraderId!),
    { refreshInterval: 15000, revalidateOnFocus: true, dedupingInterval: 8000 }
  )

  // 任一看板数据更新时刷新「最后更新」时间
  useEffect(() => {
    if (status != null || account != null || positions != null) {
      setLastUpdate(new Date().toLocaleTimeString())
    }
  }, [status, account, positions])

  // 手动刷新看板数据（供页面「刷新」按钮调用）
  const refreshDashboardData = () => {
    if (!selectedTraderId) return
    void swrMutate(`status-${selectedTraderId}`)
    void swrMutate(`account-${selectedTraderId}`)
    void swrMutate(`positions-${selectedTraderId}`)
    void swrMutate(`decisions/latest-${selectedTraderId}-${decisionsLimit}`)
    void swrMutate(`statistics-${selectedTraderId}`)
    void swrMutate(`latest-analysis-${selectedTraderId}`)
    void swrMutate(`direction-pool-${selectedTraderId}`)
    setLastUpdate(new Date().toLocaleTimeString())
  }

  const selectedTrader = (currentPage === 'simulation' ? simulationTraders : traders)?.find((t) => t.trader_id === selectedTraderId)
  const liveTradersForRadar = currentPage === 'radar' ? traders : undefined
  const simulationTradersForRadar = currentPage === 'radar' ? simulationTraders : undefined
  const isRadarDataLoading = currentPage === 'radar' && (tradersLoading || simulationTradersLoading)
  const refreshRadarData = () => {
    void swrMutate('traders')
    void swrMutate('simulation-traders')
    void swrMutate('radar-simulation-traders')
  }

  // Handle routing
  useEffect(() => {
    const handlePopState = () => {
      setRoute(window.location.pathname)
    }
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  // Set current page based on route for consistent navigation state
  useEffect(() => {
    if (route === '/competition') {
      setCurrentPage('competition')
    } else if (route === '/traders') {
      setCurrentPage('traders')
    } else if (route === '/simulation') {
      setCurrentPage('simulation')
    } else if (route === '/dashboard') {
      setCurrentPage('trader')
    }
  }, [route])

  // Show loading spinner while checking auth or config
  if (isLoading || configLoading) {
    return (
      <div
        className="min-h-screen flex items-center justify-center"
        style={{ background: '#0B0E11' }}
      >
        <div className="text-center">
          <img
            src="/icons/nofx.svg"
            alt="NoFx Logo"
            className="w-16 h-16 mx-auto mb-4 animate-pulse"
          />
          <p style={{ color: '#EAECEF' }}>{t('loading', language)}</p>
        </div>
      </div>
    )
  }

  // 已移除板块：访问旧路径时重定向到首页
  if (route === '/data' || route === '/strategy-market' || route === '/debate' || route === '/faq') {
    window.location.replace('/')
    return null
  }

  // Handle specific routes regardless of authentication
  if (route === '/login') {
    return <LoginPage />
  }
  if (route === '/register') {
    return <RegisterPage />
  }
  if (route === '/reset-password') {
    return <ResetPasswordPage />
  }
  // Show landing page for root route
  if (route === '/' || route === '') {
    return <LandingPage />
  }

  // Redirect unauthenticated users to landing page
  if (!user || !token) {
    return <LandingPage />
  }

  return (
    <div
      className="min-h-screen"
      style={{ background: '#0B0E11', color: '#EAECEF' }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage={currentPage}
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onLoginRequired={handleLoginRequired}
        onPageChange={navigateToPage}
      />

      {/* Main Content with Page Transitions */}
      <main className="min-h-screen pt-16">
        <AnimatePresence mode="wait">
          <motion.div
            key={currentPage}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -8 }}
            transition={{ duration: 0.15, ease: 'easeOut' }}
          >
            {currentPage === 'competition' ? (
              <CompetitionPage />
            ) : currentPage === 'traders' ? (
              <AITradersPage
                onTraderSelect={(traderId) => {
                  setSelectedTraderId(traderId)
                  window.history.pushState({}, '', '/dashboard')
                  setRoute('/dashboard')
                  setCurrentPage('trader')
                }}
              />
            ) : currentPage === 'simulation' ? (
              simulationTraders && simulationTraders.length > 0 && selectedTraderId && simulationTraders.some((t) => t.trader_id === selectedTraderId) ? (
                <TraderDashboardPage
                  selectedTrader={selectedTrader ?? undefined}
                  status={status}
                  account={account}
                  positions={positions}
                  decisions={decisions}
                  decisionsLimit={decisionsLimit}
                  onDecisionsLimitChange={setDecisionsLimit}
                  stats={stats}
                  lastUpdate={lastUpdate}
                  language={language}
                  traders={simulationTraders}
                  tradersError={undefined}
                  selectedTraderId={selectedTraderId}
                  onTraderSelect={(traderId) => {
                    setSelectedTraderId(traderId)
                    const trader = simulationTraders.find((t) => t.trader_id === traderId)
                    if (trader) {
                      const url = new URL(window.location.href)
                      url.pathname = '/simulation'
                      url.searchParams.set('trader', getTraderSlug(trader))
                      window.history.replaceState({}, '', url.toString())
                    }
                  }}
                  onNavigateToTraders={() => {
                    setSelectedTraderSlug(undefined)
                    setSelectedTraderId(undefined)
                    window.history.pushState({}, '', '/simulation')
                    setRoute('/simulation')
                    setCurrentPage('simulation')
                  }}
                  exchanges={simulationExchanges}
                  isSimulation
                  onRefresh={refreshDashboardData}
                  latestAnalysis={latestAnalysis}
                  directionPool={directionPool}
                />
              ) : (
                <AITradersPage
                  traders={simulationTraders}
                  isSimulation
                  onBack={() => window.history.back()}
                  onTraderSelect={(traderId) => {
                    setSelectedTraderId(traderId)
                    const trader = simulationTraders?.find((t) => t.trader_id === traderId)
                    if (trader) {
                      const url = new URL(window.location.href)
                      url.pathname = '/simulation'
                      url.searchParams.set('trader', getTraderSlug(trader))
                      window.history.pushState({}, '', url.toString())
                      setRoute('/simulation')
                    }
                  }}
                />
              )
            ) : currentPage === 'backtest' ? (
              <BacktestPage />
            ) : currentPage === 'strategy' ? (
              <StrategyStudioPage />
            ) : currentPage === 'datastats' ? (
              <DataStatisticsPage
                traders={[...(traders ?? []), ...(simulationTraders ?? [])]}
                selectedTraderId={selectedTraderId}
                onTraderSelect={(id) => setSelectedTraderId(id)}
              />
            ) : currentPage === 'radar' ? (
              <LongShortRadarPage
                liveTraders={liveTradersForRadar ?? []}
                simulationTraders={simulationTradersForRadar ?? []}
                isRadarDataLoading={isRadarDataLoading}
                onRefreshRadar={refreshRadarData}
                selectedTraderId={selectedTraderId}
                onTraderSelect={(id) => {
                  setSelectedTraderId(id)
                  window.history.replaceState({}, '', `/radar?trader=${id}`)
                }}
                status={status}
              />
            ) : (
              <TraderDashboardPage
                selectedTrader={selectedTrader}
                status={status}
                account={account}
                positions={positions}
                decisions={decisions}
                decisionsLimit={decisionsLimit}
                onDecisionsLimitChange={setDecisionsLimit}
                stats={stats}
                lastUpdate={lastUpdate}
                language={language}
                traders={traders}
                tradersError={tradersError}
                selectedTraderId={selectedTraderId}
                onTraderSelect={(traderId) => {
                  setSelectedTraderId(traderId)
                  // 更新 URL 参数（使用 slug: name-id前4位）
                  const trader = traders?.find(t => t.trader_id === traderId)
                  if (trader) {
                    const url = new URL(window.location.href)
                    url.searchParams.set('trader', getTraderSlug(trader))
                    window.history.replaceState({}, '', url.toString())
                  }
                }}
                onRefresh={refreshDashboardData}
                latestAnalysis={latestAnalysis}
                directionPool={directionPool}
                onNavigateToTraders={() => {
                  window.history.pushState({}, '', '/traders')
                  setRoute('/traders')
                  setCurrentPage('traders')
                }}
                exchanges={exchanges}
              />
            )}
          </motion.div>
        </AnimatePresence>
      </main>

      {/* Footer */}
      {(
        <footer
          className="mt-16"
          style={{ borderTop: '1px solid #2B3139', background: '#181A20' }}
        >
          <div
            className="max-w-[1920px] mx-auto px-6 py-6 text-center text-sm"
            style={{ color: '#5E6673' }}
          >
            <p>{t('footerTitle', language)}</p>
            <p className="mt-1">{t('footerWarning', language)}</p>
            <div className="mt-4 flex items-center justify-center gap-3 flex-wrap">
              {/* GitHub */}
              <a
                href={OFFICIAL_LINKS.github}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{
                  background: '#1E2329',
                  color: '#848E9C',
                  border: '1px solid #2B3139',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background = '#2B3139'
                  e.currentTarget.style.color = '#EAECEF'
                  e.currentTarget.style.borderColor = '#F0B90B'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = '#1E2329'
                  e.currentTarget.style.color = '#848E9C'
                  e.currentTarget.style.borderColor = '#2B3139'
                }}
              >
                <svg
                  width="18"
                  height="18"
                  viewBox="0 0 16 16"
                  fill="currentColor"
                >
                  <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                </svg>
                GitHub
              </a>
              {/* Twitter/X */}
              <a
                href={OFFICIAL_LINKS.twitter}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{
                  background: '#1E2329',
                  color: '#848E9C',
                  border: '1px solid #2B3139',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background = '#2B3139'
                  e.currentTarget.style.color = '#EAECEF'
                  e.currentTarget.style.borderColor = '#1DA1F2'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = '#1E2329'
                  e.currentTarget.style.color = '#848E9C'
                  e.currentTarget.style.borderColor = '#2B3139'
                }}
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                >
                  <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
                </svg>
                Twitter
              </a>
              {/* Telegram */}
              <a
                href={OFFICIAL_LINKS.telegram}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{
                  background: '#1E2329',
                  color: '#848E9C',
                  border: '1px solid #2B3139',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background = '#2B3139'
                  e.currentTarget.style.color = '#EAECEF'
                  e.currentTarget.style.borderColor = '#0088cc'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = '#1E2329'
                  e.currentTarget.style.color = '#848E9C'
                  e.currentTarget.style.borderColor = '#2B3139'
                }}
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                >
                  <path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z" />
                </svg>
                Telegram
              </a>
            </div>
          </div>
        </footer>
      )}

      {/* Login Required Overlay */}
      <LoginRequiredOverlay
        isOpen={loginOverlayOpen}
        onClose={() => setLoginOverlayOpen(false)}
        featureName={loginOverlayFeature}
      />
    </div>
  )
}


// Wrap App with providers
export default function AppWithProviders() {
  return (
    <LanguageProvider>
      <AuthProvider>
        <ConfirmDialogProvider>
          <App />
        </ConfirmDialogProvider>
      </AuthProvider>
    </LanguageProvider>
  )
}
