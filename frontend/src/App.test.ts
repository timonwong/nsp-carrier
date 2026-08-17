import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/svelte'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'

import App from './App.svelte'
import {GetSnapshot, SetProfile} from '../wailsjs/go/main/DesktopApp'
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
      id: 'awoo', displayName: 'Backend Awoo', protocolFamily: 'Awoo USB', transport: 'usb',
      supportedExtensions: ['.nsp'], wireNamespace: 'flat-basename',
      filesystemAccess: 'none', compatibleVersions: [], verifiedVersions: ['1.6.2'],
      knownIncompatibleVersions: [], adapterAvailable: true,
    },
    {
      id: 'goldleaf', displayName: 'Backend Goldleaf', protocolFamily: 'Goldleaf 0.10+', transport: 'usb',
      supportedExtensions: ['.foo'], wireNamespace: 'VIRT:/', filesystemAccess: 'read-only',
      compatibleVersions: ['0.10+'], verifiedVersions: [], knownIncompatibleVersions: [],
      adapterAvailable: true,
    },
    {
      id: 'sphaira', displayName: 'Backend Sphaira', protocolFamily: 'Sphaira SPH0', transport: 'usb',
      supportedExtensions: ['.nsp', '.msp'], wireNamespace: 'flat-basename',
      filesystemAccess: 'none', compatibleVersions: ['1.0+'], verifiedVersions: [],
      knownIncompatibleVersions: ['0.13.3 and earlier'], adapterAvailable: true,
    },
    {
      id: 'dbi', displayName: 'Backend DBI', protocolFamily: 'DBI0', transport: 'usb',
      supportedExtensions: ['.nsp', '.nsz'], wireNamespace: 'flat-basename',
      filesystemAccess: 'none', compatibleVersions: ['DBI0'], verifiedVersions: [],
      knownIncompatibleVersions: [],
      adapterAvailable: true,
    },
  ],
}

describe('installer profile UI', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(GetSnapshot).mockResolvedValue(baseSnapshot as never)
    vi.mocked(EventsOn).mockReset()
    localStorage.clear()
  })
  afterEach(() => cleanup())

  it('renders backend-provided profiles and capabilities without a frontend format matrix', async () => {
    render(App)
    const group = await screen.findByRole('group', {name: 'Installer profile'})
    const dbi = await screen.findByRole('button', {name: 'Backend DBI'})
    const goldleaf = screen.getByRole('button', {name: 'Backend Goldleaf'})
    expect([...group.querySelectorAll('button')].map((button) => button.textContent?.trim()))
      .toEqual(['Backend Awoo', 'Backend Goldleaf', 'Backend Sphaira', 'Backend DBI'])
    expect(dbi.getAttribute('aria-pressed')).toBe('false')
    expect(goldleaf.getAttribute('aria-pressed')).toBe('true')
    expect((dbi as HTMLButtonElement).disabled).toBe(false)
    expect(await screen.findByText(/FOO are eligible for Backend Goldleaf/)).toBeTruthy()
    await fireEvent.click(dbi)
    expect(SetProfile).toHaveBeenCalledWith('dbi')
  })

  it('disables profile mutation while busy', async () => {
    let sink: ((snapshot: unknown) => void) | undefined
    vi.mocked(EventsOn).mockImplementation((_event: string, callback: (...data: any[]) => void) => {
      sink = (snapshot: unknown) => callback(snapshot)
      return () => {}
    })
    render(App)
    const dbi = await screen.findByRole('button', {name: 'Backend DBI'})
    const goldleaf = screen.getByRole('button', {name: 'Backend Goldleaf'})
    sink?.({...baseSnapshot, state: 'Serving', sessionId: 'session-1'})
    await waitFor(() => expect((dbi as HTMLButtonElement).disabled).toBe(true))
    expect((goldleaf as HTMLButtonElement).disabled).toBe(true)
  })

  it('uses a compact theme menu and persists the selected appearance', async () => {
    render(App)
    const trigger = await screen.findByRole('button', {name: 'Change theme'})
    await fireEvent.click(trigger)
    expect(screen.getByRole('menuitemradio', {name: 'System'}).getAttribute('aria-checked')).toBe('true')
    await fireEvent.click(screen.getByRole('menuitemradio', {name: 'Dark'}))
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(localStorage.getItem('nsp-carrier-theme')).toBe('dark')
  })

  it('separates session state from the current serving action', async () => {
    let sink: ((snapshot: unknown) => void) | undefined
    vi.mocked(EventsOn).mockImplementation((_event: string, callback: (...data: any[]) => void) => {
      sink = (snapshot: unknown) => callback(snapshot)
      return () => {}
    })
    render(App)
    await screen.findByRole('button', {name: 'Backend Goldleaf'})
    sink?.({
      ...baseSnapshot,
      state: 'Serving', sessionId: 'session-1', canStop: true,
      items: [{
        id: 'source-1', name: 'demo.nsp', path: '/selected/demo.nsp', size: 10,
        selected: true, conflict: false, status: 'Serving', uniqueServedBytes: 4,
        wireBytes: 4, progress: 40, requested: true, validationErrors: [],
      }],
      selectedCount: 1, selectedBytes: 10, requestedBytes: 10, uniqueServedBytes: 4,
    })
    expect(await screen.findByText('Serving demo.nsp')).toBeTruthy()
    expect(screen.getByText('Serving', {selector: '.state-pill'})).toBeTruthy()
    expect(screen.getByText('Host status only')).toBeTruthy()
    expect(screen.getByText('Fully Served ≠ installed. Compatibility ≠ verified version.')).toBeTruthy()
  })

  it('labels installer-driven file states clearly', async () => {
    let sink: ((snapshot: unknown) => void) | undefined
    vi.mocked(EventsOn).mockImplementation((_event: string, callback: (...data: any[]) => void) => {
      sink = (snapshot: unknown) => callback(snapshot)
      return () => {}
    })
    render(App)
    await screen.findByRole('button', {name: 'Backend Goldleaf'})
    sink?.({
      ...baseSnapshot,
      state: 'Completed', sessionId: 'session-1',
      items: [
        {
          id: 'source-1', name: 'requested.nsp', path: '/selected/requested.nsp', size: 10,
          selected: true, conflict: false, status: 'Requested', uniqueServedBytes: 0,
          wireBytes: 0, progress: 0, requested: true, validationErrors: [],
        },
        {
          id: 'source-2', name: 'skipped.nsp', path: '/selected/skipped.nsp', size: 10,
          selected: true, conflict: false, status: 'NotRequested', uniqueServedBytes: 0,
          wireBytes: 0, progress: 0, requested: false, validationErrors: [],
        },
      ],
      selectedCount: 2, selectedBytes: 20, requestedBytes: 10,
    } as never)
    expect(await screen.findByText('Requested by Installer')).toBeTruthy()
    expect(screen.getByText('Skipped by Installer')).toBeTruthy()
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
