import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import Fastify from 'fastify';
import { registerRoutes } from '../server';

describe('POST /execute', () => {
	const app = Fastify();

	beforeAll(async () => {
		await registerRoutes(app);
		await app.ready();
	});

	afterAll(async () => {
		await app.close();
	});

	it('should execute a script and return result', async () => {
		const response = await app.inject({
			method: 'POST',
			url: '/execute',
			payload: {
				script: `
          async function check(page, context) {
            return true;
          }
        `,
				timeout: 10000,
			},
		});

		expect(response.statusCode).toBe(200);
		const body = JSON.parse(response.body);
		expect(body.ok).toBe(true);
		expect(body.result).toBe(true);
		expect(body.duration_ms).toBeGreaterThan(0);
	});

	it('should handle invalid requests', async () => {
		const response = await app.inject({
			method: 'POST',
			url: '/execute',
			payload: {},
		});

		expect(response.statusCode).toBe(400);
	});

	it('should handle script errors', async () => {
		const response = await app.inject({
			method: 'POST',
			url: '/execute',
			payload: {
				script: `
          async function check(page, context) {
            throw new Error('test error');
          }
        `,
				timeout: 10000,
			},
		});

		expect(response.statusCode).toBe(200);
		const body = JSON.parse(response.body);
		expect(body.ok).toBe(false);
		expect(body.error).toContain('test error');
	});

	it('should return screenshot when requested', async () => {
		const response = await app.inject({
			method: 'POST',
			url: '/execute',
			payload: {
				script: `
          async function check(page, context) {
            await page.goto('https://example.com', { waitUntil: 'domcontentloaded' });
            return true;
          }
        `,
				screenshot: true,
				timeout: 15000,
			},
		});

		expect(response.statusCode).toBe(200);
		const body = JSON.parse(response.body);
		expect(body.ok).toBe(true);
		expect(body.screenshot).toBeDefined();
		expect(body.screenshot.length).toBeGreaterThan(0);
	});
});
