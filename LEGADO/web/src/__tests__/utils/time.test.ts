import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { formatAge } from '../../utils/time';

describe('Time Utilities', () => {
  beforeEach(() => {
    // Fixa a data atual para os testes
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-04-16T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('should return "agora" for very recent timestamps', () => {
    const timestamp = new Date('2026-04-16T11:59:30Z').toISOString();
    expect(formatAge(timestamp)).toBe('agora');
  });

  it('should format minutes', () => {
    const timestamp = new Date('2026-04-16T11:55:00Z').toISOString();
    expect(formatAge(timestamp)).toBe('5m');
  });

  it('should format hours', () => {
    const timestamp = new Date('2026-04-16T10:00:00Z').toISOString();
    expect(formatAge(timestamp)).toBe('2h');
  });

  it('should format days', () => {
    const timestamp = new Date('2026-04-14T12:00:00Z').toISOString();
    expect(formatAge(timestamp)).toBe('2d');
  });

  it('should format months', () => {
    const timestamp = new Date('2026-01-16T12:00:00Z').toISOString();
    expect(formatAge(timestamp)).toBe('3mês');
  });

  it('should format years', () => {
    const timestamp = new Date('2024-04-16T12:00:00Z').toISOString();
    expect(formatAge(timestamp)).toBe('2a');
  });

  it('should return empty string for null/invalid input', () => {
    expect(formatAge(null)).toBe('');
    expect(formatAge('invalid-date')).toBe('');
  });
});
