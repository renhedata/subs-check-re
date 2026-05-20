import { FastifyInstance } from 'fastify';
import { executeScript } from './executor';
import { ExecuteRequest } from './types';

export async function registerRoutes(app: FastifyInstance) {
	app.post<{
		Body: ExecuteRequest;
	}>('/execute', async (request, reply) => {
		const { script, proxy, url, timeout, screenshot } = request.body;

		if (!script || typeof script !== 'string') {
			return reply.status(400).send({ error: 'script is required' });
		}

		const result = await executeScript({
			script,
			proxy,
			url,
			timeout,
			screenshot,
		});

		return reply.send(result);
	});

	app.get('/health', async () => {
		return { status: 'ok' };
	});
}
