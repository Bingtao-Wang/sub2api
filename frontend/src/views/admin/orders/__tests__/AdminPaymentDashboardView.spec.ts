import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import AdminPaymentDashboardView from '../AdminPaymentDashboardView.vue'
import type { DashboardStats, MonthlyPaymentStats } from '@/types/payment'

const { getDashboard } = vi.hoisted(() => ({
  getDashboard: vi.fn(),
}))

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: { getDashboard },
  default: { getDashboard },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const MonthlyRevenueOverviewStub = {
  name: 'MonthlyRevenueOverview',
  props: ['data'],
  template: '<div data-testid="monthly-overview-stub" />',
}

function dashboardStats(monthlySeries?: MonthlyPaymentStats[]): DashboardStats {
  return {
    today_amount: { CNY: 10 },
    total_amount: { CNY: 20 },
    today_count: 1,
    total_count: 2,
    avg_amount: { CNY: 10 },
    pending_orders: 0,
    daily_series: [],
    monthly_series: monthlySeries,
    payment_methods: [],
    top_users: {},
  }
}

async function mountView() {
  const wrapper = mount(AdminPaymentDashboardView, {
    global: {
      plugins: [createPinia()],
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        LoadingSpinner: true,
        Icon: true,
        OrderStatsCards: true,
        DailyRevenueChart: true,
        MonthlyRevenueOverview: MonthlyRevenueOverviewStub,
      },
    },
  })
  await flushPromises()
  return wrapper
}

describe('AdminPaymentDashboardView monthly revenue overview', () => {
  beforeEach(() => {
    getDashboard.mockReset()
  })

  it('passes the independent calendar-month series through to the overview', async () => {
    const monthlySeries: MonthlyPaymentStats[] = [
      { month: '2026-09', amount: { CNY: 10 }, count: 1 },
      { month: '2026-10', amount: { CNY: 20 }, count: 2 },
    ]
    getDashboard.mockResolvedValue({ data: dashboardStats(monthlySeries) })

    const wrapper = await mountView()

    expect(getDashboard).toHaveBeenCalledWith(30)
    expect(wrapper.getComponent({ name: 'MonthlyRevenueOverview' }).props('data')).toEqual(monthlySeries)
  })

  it('uses an empty series while talking to an older dashboard response', async () => {
    getDashboard.mockResolvedValue({ data: dashboardStats(undefined) })

    const wrapper = await mountView()

    expect(wrapper.getComponent({ name: 'MonthlyRevenueOverview' }).props('data')).toEqual([])
  })
})
