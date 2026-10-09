import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import MonthlyRevenueOverview from '../MonthlyRevenueOverview.vue'
import type { MonthlyPaymentStats } from '@/types/payment'

vi.mock('chart.js', () => ({
  Chart: { register: vi.fn() },
  CategoryScale: {},
  LinearScale: {},
  PointElement: {},
  LineElement: {},
  Tooltip: {},
  Legend: {},
  Filler: {},
}))

vi.mock('vue-chartjs', () => ({
  Line: {
    name: 'Line',
    props: ['data', 'options'],
    template: '<div data-testid="line-chart-stub" />',
  },
}))

const translations: Record<string, string> = {
  'payment.admin.monthlyRevenueOverview': 'Monthly Revenue Overview',
  'payment.admin.monthlyRevenueHint': 'Gross receipts before refunds',
  'payment.admin.lastTwelveCalendarMonths': 'Last 12 calendar months',
  'payment.admin.currentMonthRevenue': 'This Month Receipts',
  'payment.admin.previousMonthRevenue': 'Previous Month Receipts',
  'payment.admin.monthToDate': 'Month to date',
  'payment.admin.monthOverMonth': 'Compared with Previous Full Month',
  'payment.admin.monthlyRevenueTrend': 'Monthly Receipts Trend',
  'payment.admin.trendCurrency': 'Currency',
  'payment.admin.selectTrendCurrency': 'Select trend currency',
  'payment.admin.monthlyRevenueChartLabel': 'Monthly receipts trend for {currency}',
  'payment.admin.monthlyRevenueChartDescription': 'Use the currency selector to change the chart.',
  'payment.admin.monthlyRevenueTableCaption': 'Monthly receipts by currency',
  'payment.admin.month': 'Month',
  'payment.admin.newThisMonth': 'New',
  'payment.admin.noComparison': 'No comparable data',
  'payment.admin.noData': 'No data',
  'payment.admin.orders': 'orders',
  'payment.admin.revenue': 'Revenue',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        const message = translations[key] || key
        return Object.entries(params || {}).reduce(
          (result, [name, value]) => result.replace(`{${name}}`, value),
          message
        )
      },
      locale: { value: 'en-US' },
    }),
  }
})

function series(): MonthlyPaymentStats[] {
  return [
    { month: '2026-08', amount: { CNY: 40, USD: 80 }, count: 2 },
    { month: '2026-09', amount: { CNY: 100, EUR: 20, USD: 100 }, count: 4 },
    { month: '2026-10', amount: { CNY: 50, EUR: 20, JPY: 10, USD: 120 }, count: 5 },
  ]
}

afterEach(() => {
  document.documentElement.classList.remove('dark')
})

describe('MonthlyRevenueOverview', () => {
  it('shows current and previous gross receipts with per-currency month-over-month changes', () => {
    const wrapper = mount(MonthlyRevenueOverview, { props: { data: series() } })

    const current = wrapper.get('[data-testid="current-month-revenue"]').text()
    expect(current).toContain('This Month Receipts')
    expect(current).toContain('Oct 2026')
    expect(current).toContain('CN¥50.00')
    expect(current).toContain('€20.00')
    expect(current).toContain('¥10')
    expect(current).toContain('$120.00')
    expect(current).toContain('5 orders')

    const previous = wrapper.get('[data-testid="previous-month-revenue"]').text()
    expect(previous).toContain('Sep 2026')
    expect(previous).toContain('CN¥100.00')
    expect(previous).toContain('$100.00')
    expect(previous).toContain('4 orders')

    const growth = wrapper.get('[data-testid="month-over-month-revenue"]').text()
    expect(growth).toContain('CNY-50.0%')
    expect(growth).toContain('EUR0.0%')
    expect(growth).toContain('JPYNew')
    expect(growth).toContain('USD+20.0%')
    expect(growth).not.toMatch(/NaN|Infinity/)
  })

  it('fills a missing current-month currency with zero and reports a 100% decrease', () => {
    const wrapper = mount(MonthlyRevenueOverview, {
      props: {
        data: [
          { month: '2026-09', amount: { USD: 100 }, count: 1 },
          { month: '2026-10', amount: {}, count: 0 },
        ],
      },
    })

    expect(wrapper.get('[data-testid="current-month-revenue"]').text()).toContain('$0.00')
    expect(wrapper.get('[data-testid="previous-month-revenue"]').text()).toContain('$100.00')

    const growth = wrapper.get('[data-testid="month-over-month-revenue"]').text()
    expect(growth).toContain('USD-100.0%')
    expect(growth).not.toMatch(/NaN|Infinity/)
  })

  it('renders an em dash for an explicit-to-implicit zero comparison', () => {
    const wrapper = mount(MonthlyRevenueOverview, {
      props: {
        data: [
          { month: '2026-09', amount: { USD: 0 }, count: 0 },
          { month: '2026-10', amount: {}, count: 1 },
        ],
      },
    })

    expect(wrapper.get('[data-testid="current-month-revenue"]').text()).toContain('$0.00')
    expect(wrapper.get('[data-testid="previous-month-revenue"]').text()).toContain('$0.00')

    const growth = wrapper.get('[data-testid="month-over-month-revenue"]').text()
    expect(growth).toContain('USD—')
    expect(growth).not.toMatch(/NaN|Infinity/)
  })

  it('shows one currency at a time and updates the chart when the currency changes', async () => {
    const wrapper = mount(MonthlyRevenueOverview, { props: { data: series() } })
    const select = wrapper.get('[data-testid="monthly-revenue-currency-select"]')

    expect(select.attributes('aria-label')).toBe('Select trend currency')
    expect(select.findAll('option').map(option => option.text())).toEqual(['CNY', 'EUR', 'JPY', 'USD'])

    let data = wrapper.getComponent({ name: 'Line' }).props('data') as {
      labels: string[]
      datasets: Array<{ label: string; data: number[] }>
    }

    expect(data.labels).toEqual(['Aug 2026', 'Sep 2026', 'Oct 2026'])
    expect(data.datasets).toHaveLength(1)
    expect(data.datasets[0]).toMatchObject({ label: 'CNY Revenue', data: [40, 100, 50] })

    await select.setValue('USD')

    data = wrapper.getComponent({ name: 'Line' }).props('data') as typeof data
    expect(data.datasets).toHaveLength(1)
    expect(data.datasets[0]).toMatchObject({ label: 'USD Revenue', data: [80, 100, 120] })

    const options = wrapper.getComponent({ name: 'Line' }).props('options') as {
      scales: { y: { ticks: { callback: (value: string | number) => string } } }
      plugins: { tooltip: { callbacks: { label: (context: unknown) => string } } }
    }
    expect(options.scales.y.ticks.callback(25)).toBe('$25.00')
    expect(options.plugins.tooltip.callbacks.label({ raw: 120 })).toBe('USD: $120.00')
  })

  it('provides an accessible chart name and a complete currency-by-month data table', () => {
    const wrapper = mount(MonthlyRevenueOverview, { props: { data: series() } })
    const chart = wrapper.get('[data-testid="monthly-revenue-chart"]')

    expect(chart.attributes('role')).toBe('img')
    expect(chart.attributes('aria-label')).toBe('Monthly receipts trend for CNY')
    expect(chart.attributes('aria-describedby')).toBe('monthly-revenue-chart-description')

    const table = wrapper.get('[data-testid="monthly-revenue-accessible-table"]')
    expect(table.classes()).toContain('sr-only')
    expect(table.text()).toContain('Monthly receipts by currency')
    expect(table.text()).toContain('MonthCNYEURJPYUSD')
    expect(table.text()).toContain('Aug 2026CN¥40.00€0.00¥0$80.00')
  })

  it('updates chart contrast colors when dark mode changes and disconnects its observer', async () => {
    const disconnectSpy = vi.spyOn(MutationObserver.prototype, 'disconnect')
    const wrapper = mount(MonthlyRevenueOverview, { props: { data: series() } })

    let options = wrapper.getComponent({ name: 'Line' }).props('options') as {
      scales: { x: { ticks: { color: string } }; y: { title: { color: string } } }
      plugins: {
        legend: { labels: { color: string } }
        tooltip: { backgroundColor: string; titleColor: string; bodyColor: string }
      }
    }
    expect(options.scales.x.ticks.color).toBe('#374151')
    expect(options.plugins.tooltip.backgroundColor).toBe('#ffffff')

    document.documentElement.classList.add('dark')
    await new Promise(resolve => setTimeout(resolve, 0))

    options = wrapper.getComponent({ name: 'Line' }).props('options') as typeof options
    expect(options.scales.x.ticks.color).toBe('#e5e7eb')
    expect(options.scales.y.title.color).toBe('#e5e7eb')
    expect(options.plugins.legend.labels.color).toBe('#e5e7eb')
    expect(options.plugins.tooltip).toMatchObject({
      backgroundColor: '#111827',
      titleColor: '#e5e7eb',
      bodyColor: '#e5e7eb',
    })

    wrapper.unmount()
    expect(disconnectSpy).toHaveBeenCalled()
    disconnectSpy.mockRestore()
  })

  it('renders an explicit empty state without a chart', () => {
    const wrapper = mount(MonthlyRevenueOverview, {
      props: {
        data: [
          { month: '2026-09', amount: {}, count: 0 },
          { month: '2026-10', amount: {}, count: 0 },
        ],
      },
    })

    expect(wrapper.text()).toContain('No data')
    expect(wrapper.findComponent({ name: 'Line' }).exists()).toBe(false)
  })
})
