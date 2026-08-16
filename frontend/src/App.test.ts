import {cleanup, render, screen, waitFor} from '@testing-library/svelte'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'

import App from './App.svelte'
import {GetSnapshot} from '../wailsjs/go/main/DesktopApp'
import {EventsOn} from '../wailsjs/runtime/runtime'

vi.mock('../wailsjs/go/main/DesktopApp', () => ({
  ChooseFiles: vi.fn(), ChooseFolder: vi.fn(), ClearQueue: vi.fn(),
  GetSnapshot: vi.fn(), Remove: vi.fn(), SetAllSelected: vi.fn(),
  SetProfile: vi.fn(), SetSelected: vi.fn(), Start: vi.fn(), Stop: vi.fn(),
}))
vi.mock('../wailsjs/runtime/runtime', () => ({EventsOff: vi.fn(), EventsOn: vi.fn()}))

const baseSnapshot = {
  state: 'Idle', sessionId: '', items: [], logs: [], selectedCount: 0,
  selectedBytes: 0, requestedBytes: 0, uniqueServedBytes: 0, wireBytes: 0,
  overallProgress: 0, canStart: false, canStop: false, hasConflict: false,
  lastError: '', activeProfile: 'goldleaf', validationErrors: [],
  profiles: [
    {
      id: 'dbi', displayName: 'Backend DBI', protocolFamily: 'DBI0', transport: 'usb',
      supportedExtensions: ['.nsp', '.nsz'], wireNamespace: 'flat-basename',
      filesystemAccess: 'none', compatibleVersions: ['DBI0'], verifiedVersions: [],
      knownIncompatibleVersions: [],
      adapterAvailable: true,
    },
    {
      id: 'goldleaf', displayName: 'Backend Goldleaf', protocolFamily: 'Goldleaf 0.10+', transport: 'usb',
      supportedExtensions: ['.foo'], wireNamespace: 'VIRT:/', filesystemAccess: 'read-only',
      compatibleVersions: ['0.10+'], verifiedVersions: [], knownIncompatibleVersions: [],
      adapterAvailable: true,
    },
  ],
}

describe('installer profile UI', () => {
  beforeEach(() => {
    vi.mocked(GetSnapshot).mockResolvedValue(baseSnapshot as never)
    vi.mocked(EventsOn).mockReset()
  })
  afterEach(() => cleanup())

  it('renders backend-provided profiles and capabilities without a frontend format matrix', async () => {
    render(App)
    const selector = await screen.findByRole('combobox', {name: 'Installer profile'})
    expect(await screen.findByRole('option', {name: 'Backend DBI'})).toBeTruthy()
    expect(screen.getByRole('option', {name: 'Backend Goldleaf'})).toBeTruthy()
    expect((selector as HTMLSelectElement).disabled).toBe(false)
    expect(await screen.findByText(/FOO are eligible for Backend Goldleaf/)).toBeTruthy()
  })

  it('disables profile mutation while busy', async () => {
    let sink: ((snapshot: unknown) => void) | undefined
    vi.mocked(EventsOn).mockImplementation((_event: string, callback: (...data: any[]) => void) => {
      sink = (snapshot: unknown) => callback(snapshot)
      return () => {}
    })
    render(App)
    const selector = await screen.findByRole('combobox', {name: 'Installer profile'})
    sink?.({...baseSnapshot, state: 'Serving', sessionId: 'session-1'})
    await waitFor(() => expect((selector as HTMLSelectElement).disabled).toBe(true))
  })

  it('shows item-specific validation without deselecting the file', async () => {
    const validationError = {
      sourceId: 'source-1', name: 'compressed.nsz', code: 'unsupported-extension',
      message: 'Backend Goldleaf does not support .nsz files',
    }
    vi.mocked(GetSnapshot).mockResolvedValue({
      ...baseSnapshot,
      items: [{
        id: 'source-1', name: 'compressed.nsz', path: '/selected/compressed.nsz', size: 10,
        selected: true, conflict: false, status: 'Queued', uniqueServedBytes: 0,
        wireBytes: 0, progress: 0, requested: false, validationErrors: [validationError],
      }],
      selectedCount: 1, selectedBytes: 10, validationErrors: [validationError],
    } as never)
    render(App)
    expect(await screen.findByText('Unsupported selection')).toBeTruthy()
    expect(screen.getByText(validationError.message)).toBeTruthy()
    expect((screen.getByRole('checkbox', {name: 'Select compressed.nsz'}) as HTMLInputElement).checked).toBe(true)
  })
})
