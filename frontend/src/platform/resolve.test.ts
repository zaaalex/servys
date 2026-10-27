import { describe, expect, it } from 'vitest'
import { readBridge, resolveLaunch } from './resolve'

describe('resolveLaunch', () => {
  it('reads Telegram init data from the fragment and keeps it encoded', () => {
    expect(resolveLaunch({
      hash: '#tgWebAppData=foo%3Dbar+baz&tgWebAppVersion=8',
      search: '',
    })).toEqual({ id: 'telegram', initData: 'foo%3Dbar+baz' })
  })

  it('does not treat tgWebAppData as MAX WebAppData', () => {
    expect(resolveLaunch({
      hash: '#tgWebAppData=abc',
      search: '',
    })).toEqual({ id: 'telegram', initData: 'abc' })
    expect(resolveLaunch({
      hash: '#xWebAppData=1',
      search: '',
    })).toEqual({ id: 'unknown', initData: '' })
  })

  it('reads MAX from the fragment', () => {
    expect(resolveLaunch({
      hash: '#WebAppData=max%20data&WebAppPlatform=ios',
      search: '',
    })).toEqual({ id: 'max', initData: 'max%20data' })
  })

  it('reads a single key from the query', () => {
    expect(resolveLaunch({
      hash: '',
      search: '?tgWebAppData=from-query',
    })).toEqual({ id: 'telegram', initData: 'from-query' })
    expect(resolveLaunch({
      hash: '',
      search: '?WebAppData=from-query',
    })).toEqual({ id: 'max', initData: 'from-query' })
  })

  it('prefers the fragment value when the same key is also in the query', () => {
    expect(resolveLaunch({
      hash: '#tgWebAppData=from-hash',
      search: '?tgWebAppData=from-query',
    })).toEqual({ id: 'telegram', initData: 'from-hash' })
  })

  it('does not look at the other platform in the query after the fragment selected one', () => {
    expect(resolveLaunch({
      hash: '#tgWebAppData=tg',
      search: '?WebAppData=max',
    })).toEqual({ id: 'telegram', initData: 'tg' })
    expect(resolveLaunch({
      hash: '#WebAppData=max',
      search: '?tgWebAppData=tg',
    })).toEqual({ id: 'max', initData: 'max' })
  })

  it('returns unknown when both keys are in the fragment', () => {
    expect(resolveLaunch({
      hash: '#tgWebAppData=tg&WebAppData=max',
      search: '?tgWebAppData=only',
    })).toEqual({ id: 'unknown', initData: '' })
  })

  it('returns unknown when both keys are in the query and the fragment selected nothing', () => {
    expect(resolveLaunch({
      hash: '#tgWebAppVersion=8',
      search: '?tgWebAppData=tg&WebAppData=max',
    })).toEqual({ id: 'unknown', initData: '' })
  })

  it('returns unknown when neither key is present', () => {
    expect(resolveLaunch({ hash: '', search: '' })).toEqual({ id: 'unknown', initData: '' })
  })

  it('ignores empty values and falls through to the query', () => {
    expect(resolveLaunch({
      hash: '#tgWebAppData=&WebAppData=',
      search: '?WebAppData=max-value',
    })).toEqual({ id: 'max', initData: 'max-value' })
  })

  it('uses the only non-empty key in the fragment', () => {
    expect(resolveLaunch({
      hash: '#tgWebAppData=&WebAppData=max',
      search: '?tgWebAppData=tg',
    })).toEqual({ id: 'max', initData: 'max' })
  })

  it('accepts a fragment and query without the leading mark', () => {
    expect(resolveLaunch({
      hash: 'tgWebAppData=plain',
      search: 'unused=1',
    })).toEqual({ id: 'telegram', initData: 'plain' })
  })

  it('keeps the first non-empty value of a repeated key', () => {
    expect(resolveLaunch({
      hash: '#tgWebAppData=&tgWebAppData=second',
      search: '',
    })).toEqual({ id: 'telegram', initData: 'second' })
  })
})

describe('readBridge', () => {
  it('reads Telegram initData from the SDK', () => {
    expect(readBridge({ Telegram: { WebApp: { initData: 'tg-signed' } } })).toEqual({
      id: 'telegram',
      initData: 'tg-signed',
    })
  })

  it('reads MAX initData from the SDK', () => {
    expect(readBridge({ WebApp: { initData: 'max-signed' } })).toEqual({
      id: 'max',
      initData: 'max-signed',
    })
  })

  it('rejects an empty bridge and both messengers at once', () => {
    expect(readBridge({})).toEqual({ id: 'unknown', initData: '' })
    expect(readBridge({
      Telegram: { WebApp: { initData: 'tg' } },
      WebApp: { initData: 'max' },
    })).toEqual({ id: 'unknown', initData: '' })
  })
})
