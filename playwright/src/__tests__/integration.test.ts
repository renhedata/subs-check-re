import { describe, it, expect } from 'vitest';

const PLAYWRIGHT_SERVICE_URL = process.env.PLAYWRIGHT_SERVICE_URL || 'http://localhost:3000';
const SKIP_INTEGRATION = process.env.SKIP_INTEGRATION_TESTS === 'true';

describe('Integration: Playwright service', () => {
	it('should execute Netflix-like check script', async () => {
		if (SKIP_INTEGRATION) {
			// eslint-disable-next-line no-console
			console.log('Skipping integration test: SKIP_INTEGRATION_TESTS is true');
			return;
		}

		const script = `
			async function check(page, context) {
				await page.goto('https://www.netflix.com/title/81280792', {
					waitUntil: 'domcontentloaded'
				});

				// Wait for potential CF challenge
				await page.waitForTimeout(3000);

				const text = await page.textContent('body');
				return !text.includes('Oh no!');
			}
		`;

		const response = await fetch(`${PLAYWRIGHT_SERVICE_URL}/execute`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ script, timeout: 30000 }),
		});

		const result = await response.json();
		expect(result.ok).toBe(true);
		expect(typeof result.result).toBe('boolean');
	}, 60000);
});
