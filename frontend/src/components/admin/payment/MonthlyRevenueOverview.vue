<template>
  <section class="card p-4" data-testid="monthly-revenue-overview">
    <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('payment.admin.monthlyRevenueOverview') }}
        </h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('payment.admin.monthlyRevenueHint') }}
        </p>
      </div>
      <span class="w-fit rounded-full bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
        {{ t('payment.admin.lastTwelveCalendarMonths') }}
      </span>
    </div>

    <div v-if="!hasData" class="flex h-40 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('payment.admin.noData') }}
    </div>

    <template v-else>
      <div class="mt-4 grid grid-cols-1 gap-3 md:grid-cols-3">
        <article class="rounded-xl border border-green-100 bg-green-50/70 p-4 dark:border-green-900/40 dark:bg-green-950/20" data-testid="current-month-revenue">
          <div class="flex items-center justify-between gap-2">
            <p class="text-xs font-medium text-green-700 dark:text-green-300">
              {{ t('payment.admin.currentMonthRevenue') }}
            </p>
            <span class="text-xs text-green-600/80 dark:text-green-400/80">{{ formatMonth(currentMonth.month) }}</span>
          </div>
          <div class="mt-2 space-y-1">
            <p v-for="[currency, amount] in currentMonthAmounts" :key="currency" class="text-xl font-bold text-gray-900 dark:text-white">
              {{ formatMoney(currency, amount) }}
            </p>
            <p v-if="!currentMonthAmounts.length" class="text-xl font-bold text-gray-900 dark:text-white">—</p>
          </div>
          <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
            {{ t('payment.admin.monthToDate') }} · {{ currentMonth.count }} {{ t('payment.admin.orders') }}
          </p>
        </article>

        <article class="rounded-xl border border-blue-100 bg-blue-50/70 p-4 dark:border-blue-900/40 dark:bg-blue-950/20" data-testid="previous-month-revenue">
          <div class="flex items-center justify-between gap-2">
            <p class="text-xs font-medium text-blue-700 dark:text-blue-300">
              {{ t('payment.admin.previousMonthRevenue') }}
            </p>
            <span class="text-xs text-blue-600/80 dark:text-blue-400/80">{{ formatMonth(previousMonth.month) }}</span>
          </div>
          <div class="mt-2 space-y-1">
            <p v-for="[currency, amount] in previousMonthAmounts" :key="currency" class="text-xl font-bold text-gray-900 dark:text-white">
              {{ formatMoney(currency, amount) }}
            </p>
            <p v-if="!previousMonthAmounts.length" class="text-xl font-bold text-gray-900 dark:text-white">—</p>
          </div>
          <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
            {{ previousMonth.count }} {{ t('payment.admin.orders') }}
          </p>
        </article>

        <article class="rounded-xl border border-violet-100 bg-violet-50/70 p-4 dark:border-violet-900/40 dark:bg-violet-950/20" data-testid="month-over-month-revenue">
          <p class="text-xs font-medium text-violet-700 dark:text-violet-300">
            {{ t('payment.admin.monthOverMonth') }}
          </p>
          <div v-if="growthRows.length" class="mt-2 space-y-2">
            <div v-for="row in growthRows" :key="row.currency" class="flex items-center justify-between gap-3">
              <span class="text-xs font-semibold text-gray-500 dark:text-gray-400">{{ row.currency }}</span>
              <span :class="['text-lg font-bold', growthClass(row.trend)]">
                {{ formatGrowth(row) }}
              </span>
            </div>
          </div>
          <p v-else class="mt-3 text-sm text-gray-500 dark:text-gray-400">
            {{ t('payment.admin.noComparison') }}
          </p>
        </article>
      </div>

      <div class="mt-5 border-t border-gray-100 pt-4 dark:border-dark-700">
        <div class="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <h4 class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
            {{ t('payment.admin.monthlyRevenueTrend') }}
          </h4>
          <label v-if="availableCurrencies.length" class="flex items-center gap-2 text-xs font-medium text-gray-600 dark:text-gray-300">
            <span>{{ t('payment.admin.trendCurrency') }}</span>
            <select
              v-model="selectedCurrency"
              class="input min-w-24 py-1 text-xs"
              data-testid="monthly-revenue-currency-select"
              :aria-label="t('payment.admin.selectTrendCurrency')"
            >
              <option v-for="currency in availableCurrencies" :key="currency" :value="currency">
                {{ currency }}
              </option>
            </select>
          </label>
        </div>
        <div
          class="h-64"
          data-testid="monthly-revenue-chart"
          role="img"
          :aria-label="chartAccessibleLabel"
          aria-describedby="monthly-revenue-chart-description"
        >
          <Line v-if="chartData" :data="chartData" :options="chartOptions" />
        </div>
        <p id="monthly-revenue-chart-description" class="sr-only">
          {{ t('payment.admin.monthlyRevenueChartDescription') }}
        </p>
        <div class="sr-only" data-testid="monthly-revenue-accessible-table">
          <table>
            <caption>{{ t('payment.admin.monthlyRevenueTableCaption') }}</caption>
            <thead>
              <tr>
                <th scope="col">{{ t('payment.admin.month') }}</th>
                <th v-for="currency in availableCurrencies" :key="currency" scope="col">
                  {{ currency }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in data" :key="item.month">
                <th scope="row">{{ formatMonth(item.month) }}</th>
                <td v-for="currency in availableCurrencies" :key="currency">
                  {{ formatMoney(currency, item.amount[currency] ?? 0) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import type { TooltipItem } from 'chart.js'
import { Line } from 'vue-chartjs'
import type { CurrencyAmounts, MonthlyPaymentStats } from '@/types/payment'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

type GrowthTrend = 'up' | 'down' | 'flat' | 'new'

interface GrowthRow {
  currency: string
  percent: number | null
  trend: GrowthTrend
}

const props = defineProps<{
  data: MonthlyPaymentStats[]
}>()

const { t, locale } = useI18n()
const emptyMonth: MonthlyPaymentStats = { month: '', amount: {}, count: 0 }

const currentMonth = computed(() => props.data[props.data.length - 1] || emptyMonth)
const previousMonth = computed(() => props.data[props.data.length - 2] || emptyMonth)
const hasData = computed(() => props.data.some(item => item.count > 0 || Object.values(item.amount).some(amount => amount !== 0)))
const comparisonCurrencies = computed(() => [...new Set([
  ...Object.keys(currentMonth.value.amount),
  ...Object.keys(previousMonth.value.amount),
])].sort())
const currentMonthAmounts = computed(() => amountsForCurrencies(currentMonth.value.amount, comparisonCurrencies.value))
const previousMonthAmounts = computed(() => amountsForCurrencies(previousMonth.value.amount, comparisonCurrencies.value))
const availableCurrencies = computed(() => [...new Set(props.data.flatMap(item => Object.keys(item.amount)))].sort())
const selectedCurrency = ref('')
const isDarkMode = ref(false)
let themeObserver: MutationObserver | undefined

watch(availableCurrencies, (currencies) => {
  if (!currencies.includes(selectedCurrency.value)) {
    selectedCurrency.value = currencies[0] || ''
  }
}, { immediate: true })

onMounted(() => {
  updateTheme()
  themeObserver = new MutationObserver(updateTheme)
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})

onBeforeUnmount(() => {
  themeObserver?.disconnect()
})

const growthRows = computed<GrowthRow[]>(() => {
  return comparisonCurrencies.value.map((currency) => {
    const current = currentMonth.value.amount[currency] ?? 0
    const previous = previousMonth.value.amount[currency] ?? 0
    if (previous === 0) {
      return { currency, percent: null, trend: current > 0 ? 'new' : 'flat' }
    }
    const percent = ((current - previous) / previous) * 100
    return {
      currency,
      percent,
      trend: Math.abs(percent) < 0.05 ? 'flat' : percent > 0 ? 'up' : 'down',
    }
  })
})

const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? 'rgba(107, 114, 128, 0.35)' : 'rgba(156, 163, 175, 0.3)',
  line: '#10b981',
  fill: 'rgba(16, 185, 129, 0.12)',
  tooltipBackground: isDarkMode.value ? '#111827' : '#ffffff',
  tooltipBorder: isDarkMode.value ? '#4b5563' : '#d1d5db',
}))

const chartData = computed(() => {
  if (!selectedCurrency.value) return null

  return {
    labels: props.data.map(item => formatMonth(item.month)),
    datasets: [
      {
        label: `${selectedCurrency.value} ${t('payment.admin.revenue')}`,
        data: props.data.map(item => item.amount[selectedCurrency.value] ?? 0),
        borderColor: chartColors.value.line,
        backgroundColor: chartColors.value.fill,
        fill: true,
        tension: 0.3,
        pointRadius: 3,
        pointHoverRadius: 5,
      },
    ],
  }
})

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  scales: {
    x: {
      grid: { color: chartColors.value.grid },
      ticks: { color: chartColors.value.text },
    },
    y: {
      beginAtZero: true,
      grid: { color: chartColors.value.grid },
      ticks: {
        color: chartColors.value.text,
        callback: (value: string | number) => formatMoney(selectedCurrency.value, Number(value)),
      },
      title: {
        display: true,
        text: `${t('payment.admin.revenue')} (${selectedCurrency.value})`,
        color: chartColors.value.text,
      },
    },
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: { color: chartColors.value.text },
    },
    tooltip: {
      backgroundColor: chartColors.value.tooltipBackground,
      borderColor: chartColors.value.tooltipBorder,
      borderWidth: 1,
      titleColor: chartColors.value.text,
      bodyColor: chartColors.value.text,
      callbacks: {
        label: (context: TooltipItem<'line'>) => `${selectedCurrency.value}: ${formatMoney(selectedCurrency.value, Number(context.raw))}`,
      },
    },
  },
}))

const chartAccessibleLabel = computed(() => t('payment.admin.monthlyRevenueChartLabel', {
  currency: selectedCurrency.value,
}))

function amountsForCurrencies(amounts: CurrencyAmounts, currencies: string[]): [string, number][] {
  return currencies.map(currency => [currency, amounts[currency] ?? 0])
}

function formatMoney(currency: string, amount: number): string {
  return new Intl.NumberFormat(locale.value, { style: 'currency', currency }).format(amount)
}

function formatMonth(month: string): string {
  const [year, monthNumber] = month.split('-').map(Number)
  if (!year || !monthNumber) return '—'
  return new Intl.DateTimeFormat(locale.value, { year: 'numeric', month: 'short' }).format(new Date(year, monthNumber - 1, 1))
}

function formatGrowth(row: GrowthRow): string {
  if (row.trend === 'new') return t('payment.admin.newThisMonth')
  if (row.percent == null) return '—'
  return new Intl.NumberFormat(locale.value, {
    style: 'percent',
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
    signDisplay: 'exceptZero',
  }).format(row.percent / 100)
}

function growthClass(trend: GrowthTrend): string {
  if (trend === 'up' || trend === 'new') return 'text-green-600 dark:text-green-400'
  if (trend === 'down') return 'text-red-600 dark:text-red-400'
  return 'text-gray-600 dark:text-gray-300'
}

function updateTheme(): void {
  isDarkMode.value = document.documentElement.classList.contains('dark')
}
</script>
