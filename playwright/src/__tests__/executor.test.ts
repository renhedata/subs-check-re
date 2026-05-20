import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { executeScript } from '../executor';

describe('executeScript', () => {
  it('should execute a simple script that returns true', async () => {
    const script = `
      async function check(page, context) {
        return true;
      }
    `;
    const result = await executeScript({ script, timeout: 10000 });
    expect(result.ok).toBe(true);
    expect(result.result).toBe(true);
  });

  it('should execute a simple script that returns false', async () => {
    const script = `
      async function check(page, context) {
        return false;
      }
    `;
    const result = await executeScript({ script, timeout: 10000 });
    expect(result.ok).toBe(true);
    expect(result.result).toBe(false);
  });

  it('should capture console logs', async () => {
    const script = `
      async function check(page, context) {
        await page.evaluate(() => { console.log('test log'); });
        return true;
      }
    `;
    const result = await executeScript({ script, timeout: 10000 });
    expect(result.logs.some((l: string) => l.includes('test log'))).toBe(true);
  });

  it('should handle script errors gracefully', async () => {
    const script = `
      async function check(page, context) {
        throw new Error('script error');
      }
    `;
    const result = await executeScript({ script, timeout: 10000 });
    expect(result.ok).toBe(false);
    expect(result.error).toContain('script error');
  });

  it('should respect timeout', async () => {
    const script = `
      async function check(page, context) {
        await page.waitForTimeout(100000);
        return true;
      }
    `;
    const result = await executeScript({ script, timeout: 1000 });
    expect(result.ok).toBe(false);
    expect(result.error?.toLowerCase()).toContain('timed out');
  });

  it('should navigate to URL and check content', async () => {
    const script = `
      async function check(page, context) {
        await page.goto('https://example.com', { waitUntil: 'domcontentloaded' });
        const title = await page.title();
        return title.includes('Example');
      }
    `;
    const result = await executeScript({ script, timeout: 15000 });
    expect(result.ok).toBe(true);
    expect(result.result).toBe(true);
    expect(result.title).toContain('Example');
  }, 20000);
});
