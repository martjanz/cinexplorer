import { describe, expect, it } from 'vitest'
import { keyAction } from './keys.js'

const key = (k, extra = {}) => ({ key: k, target: { tagName: 'BODY' }, ...extra })

describe('keyAction', () => {
  it.each([
    ['ArrowDown', { type: 'next' }],
    ['ArrowUp', { type: 'prev' }],
    ['1', { type: 'pick', index: 0 }],
    ['5', { type: 'pick', index: 4 }],
    ['/', { type: 'search' }],
    ['n', { type: 'ignore' }],
    ['N', { type: 'ignore' }],
    ['e', { type: 'extra' }],
    ['6', null],
    ['x', null],
  ])('%s', (k, want) => {
    expect(keyAction(key(k))).toEqual(want)
  })
  it('ignores typing and modifiers', () => {
    expect(keyAction(key('n', { target: { tagName: 'INPUT' } }))).toBeNull()
    expect(keyAction(key('1', { target: { tagName: 'DIV', isContentEditable: true } }))).toBeNull()
    expect(keyAction(key('1', { ctrlKey: true }))).toBeNull()
  })
})
