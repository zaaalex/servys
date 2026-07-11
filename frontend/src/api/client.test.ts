import { describe, expect, it } from 'vitest'
import { normalizeVinResponse } from './client'

describe('VIN response normalization', () => {
  it('builds signature.bodyType for LJD3AA293L0051345 backend response', () => {
    const result = normalizeVinResponse({
      vin: 'LJD3AA293L0051345',
      make: 'KIA', model: 'K3', year: 2020,
      engine_cc: 1353, power_hp: 130,
      identification_source: 'fixture',
    })

    expect(result.vin).toBe('LJD3AA293L0051345')
    expect(result.signature).toMatchObject({
      make: 'KIA', model: 'K3', year: 2020,
      engineDisplacementCc: 1353, powerHp: 130,
      bodyType: 'sedan',
    })
  })
})
