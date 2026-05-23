import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { parseMetrics } from '../../src/lib/promParse'

const FIXTURE = readFileSync(
  resolve(__dirname, 'fixtures/metrics-sample.txt'),
  'utf8',
)

describe('parseMetrics', () => {
  it('returns [] for empty input', () => {
    expect(parseMetrics('')).toEqual([])
  })

  it('skips comments and empty lines', () => {
    const samples = parseMetrics('# HELP foo bar\n# TYPE foo counter\n\nfoo 1')
    expect(samples).toHaveLength(1)
    expect(samples[0]).toMatchObject({ name: 'foo', value: 1, type: 'counter' })
  })

  it('parses unlabeled gauges', () => {
    const samples = parseMetrics('# TYPE x gauge\nx 5')
    expect(samples[0]).toMatchObject({ name: 'x', value: 5, type: 'gauge', labels: {} })
  })

  it('parses labels including multiple key/value pairs', () => {
    const samples = parseMetrics(
      '# TYPE e counter\ne{protocol="task",reason="reset"} 7',
    )
    expect(samples[0].labels).toEqual({ protocol: 'task', reason: 'reset' })
    expect(samples[0].value).toBe(7)
  })

  it('parses scientific notation', () => {
    const samples = parseMetrics('# TYPE m gauge\nm 1.4892e+08')
    expect(samples[0].value).toBeCloseTo(1.4892e8)
  })

  it('parses +Inf as expected for histogram bucket le label', () => {
    const samples = parseMetrics(
      '# TYPE h histogram\nh_bucket{le="+Inf"} 142',
    )
    expect(samples[0].labels.le).toBe('+Inf')
    expect(samples[0].value).toBe(142)
  })

  it('skips malformed lines without aborting', () => {
    const text = '# TYPE a counter\na 1\nthis is not a valid line\na 2'
    const samples = parseMetrics(text)
    expect(samples.map((s) => s.value)).toEqual([1, 2])
  })

  it('parses the full /metrics fixture', () => {
    const samples = parseMetrics(FIXTURE)
    const names = new Set(samples.map((s) => s.name))
    expect(names.has('agentfm_tasks_total')).toBe(true)
    expect(names.has('agentfm_task_duration_seconds_bucket')).toBe(true)
    expect(names.has('agentfm_task_duration_seconds_sum')).toBe(true)
    expect(names.has('agentfm_task_duration_seconds_count')).toBe(true)
    expect(names.has('agentfm_workers_online')).toBe(true)
    expect(names.has('agentfm_stream_errors_total')).toBe(true)
    expect(names.has('process_resident_memory_bytes')).toBe(true)
    expect(names.has('go_goroutines')).toBe(true)

    const ok = samples.find(
      (s) => s.name === 'agentfm_tasks_total' && s.labels.status === 'ok',
    )
    expect(ok?.value).toBe(142)

    const online = samples.find((s) => s.name === 'agentfm_workers_online')
    expect(online?.value).toBe(5)
    expect(online?.type).toBe('gauge')

    const bucket = samples.find(
      (s) =>
        s.name === 'agentfm_task_duration_seconds_bucket' &&
        s.labels.le === '60',
    )
    expect(bucket?.value).toBe(128)
    expect(bucket?.type).toBe('histogram')
  })
})
