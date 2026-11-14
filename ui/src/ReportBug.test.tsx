import { beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import '@testing-library/jest-dom/vitest'
import ReportBug from './ReportBug'

const reportBug = vi.fn(async () => ({
  ok: true as const,
  feature: { id: 'B-0001', title: 'Salvar quebra', state: 'approved' },
}))

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  reportBug: (...a: unknown[]) => (reportBug as (...x: unknown[]) => unknown)(...a),
}))

beforeEach(() => {
  cleanup()
  reportBug.mockClear()
  window.location.hash = ''
})

describe('ReportBug', () => {
  // T-0006 critério 5: título + os três campos do relato; o envio cria a B-
  // e leva à página dela.
  it('envia o relato de três campos e abre a B- criada', async () => {
    render(<ReportBug />)
    const submit = screen.getByRole('button', { name: /Reportar correção/ })
    expect(submit).toBeDisabled()
    fireEvent.change(screen.getByLabelText(/Título/), { target: { value: 'Salvar quebra' } })
    fireEvent.change(screen.getByLabelText(/Contexto/), { target: { value: 'O salvar falha' } })
    fireEvent.change(screen.getByLabelText(/Como reproduzir/), { target: { value: '1. salvar' } })
    fireEvent.change(screen.getByLabelText(/Esperado × obtido/), { target: { value: 'salvar ok' } })
    expect(submit).toBeEnabled()
    fireEvent.click(submit)
    await waitFor(() =>
      expect(reportBug).toHaveBeenCalledWith('Salvar quebra', 'O salvar falha', '1. salvar', 'salvar ok'),
    )
    await waitFor(() => expect(window.location.hash).toBe('#/f/B-0001'))
  })
})
